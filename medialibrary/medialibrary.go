package medialibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	_ "image/gif" // Register GIF decoder
	"image/jpeg"
	_ "image/jpeg" // Register JPEG decoder
	"image/png"
	_ "image/png" // Register PNG decoder
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vortechron/go-medialibrary/conversion"
	"github.com/vortechron/go-medialibrary/models"
	"github.com/vortechron/go-medialibrary/storage"
)

type MediaLibrary interface {
	AddMediaFromURL(ctx context.Context, url string, collection string, options ...Option) (*models.Media, error)

	AddMediaFromURLToModel(ctx context.Context, url string, modelType string, modelID uint64, collection string, options ...Option) (*models.Media, error)

	AddMediaFromDisk(ctx context.Context, filePath string, collection string, options ...Option) (*models.Media, error)

	AddMediaFromDiskToDisk(ctx context.Context, sourceDisk string, sourcePath string, targetDisk string, collection string, options ...Option) (*models.Media, error)

	CopyMediaToDisk(ctx context.Context, media *models.Media, targetDisk string) (*models.Media, error)

	MoveMediaToDisk(ctx context.Context, media *models.Media, targetDisk string) (*models.Media, error)

	PerformConversions(ctx context.Context, media *models.Media, conversionNames ...string) error

	GenerateResponsiveImages(ctx context.Context, media *models.Media, conversionNames ...string) error

	GetURLForMedia(media *models.Media) string

	GetURLForMediaConversion(media *models.Media, conversionName string) string

	GetURLForResponsiveImage(media *models.Media, conversionName string, width int) string

	GetMediaUrl(media *models.Media) string

	GetMediaConversionUrl(media *models.Media, conversionName string) string

	GetMediaResponsiveImageUrl(media *models.Media, conversionName string, width int) string

	GetMediaRepository() MediaRepository

	GetMediaForModel(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error)

	GetMediaForModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error)
}

type MediaRepository interface {
	Save(ctx context.Context, media *models.Media) error

	FindByID(ctx context.Context, id uint64) (*models.Media, error)

	Delete(ctx context.Context, media *models.Media) error
}

type Option func(*Options)

type Options struct {
	DefaultDisk              string
	ConversionsDisk          string
	AutoGenerateConversions  bool
	PerformConversions       []string
	GenerateResponsiveImages []string
	CustomProperties         map[string]interface{}
	ModelType                string
	ModelID                  uint64
	PathGeneratorPrefix      string
	Name                     string
}

type DefaultMediaLibrary struct {
	diskManager    *storage.DiskManager
	transformer    conversion.Transformer
	repository     MediaRepository
	defaultOptions *Options
	pathGenerator  PathGenerator
}

type PathGenerator interface {
	GetPath(media *models.Media) string

	GetPathForConversion(media *models.Media, conversionName string) string

	GetPathForResponsiveImage(media *models.Media, conversionName string, width int) string
}

type DefaultPathGenerator struct {
	prefix string
}

func (p *DefaultPathGenerator) getBasePath(media *models.Media) string {
	return fmt.Sprintf("%s/%d/", p.prefix, media.ID)
}

func (p *DefaultPathGenerator) GetPath(media *models.Media) string {
	return fmt.Sprintf("%s/%s",
		p.getBasePath(media),
		media.FileName)
}

func (p *DefaultPathGenerator) GetPathForConversion(media *models.Media, conversionName string) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return fmt.Sprintf("%s/%s/conversions/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+ext)
}

func (p *DefaultPathGenerator) GetPathForResponsiveImage(media *models.Media, conversionName string, width int) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return fmt.Sprintf("%s/%s/responsive-images/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+"-"+fmt.Sprintf("%d", width)+ext)
}

func NewDefaultMediaLibrary(
	diskManager *storage.DiskManager,
	transformer conversion.Transformer,
	repository MediaRepository,
	options ...Option,
) *DefaultMediaLibrary {
	opts := &Options{
		DefaultDisk:      "s3",
		ConversionsDisk:  "s3",
		CustomProperties: make(map[string]interface{}),
	}

	for _, opt := range options {
		opt(opts)
	}

	return &DefaultMediaLibrary{
		diskManager:    diskManager,
		transformer:    transformer,
		repository:     repository,
		defaultOptions: opts,
		pathGenerator: &DefaultPathGenerator{
			prefix: opts.PathGeneratorPrefix,
		},
	}
}

func WithDefaultDisk(disk string) Option {
	return func(o *Options) {
		o.DefaultDisk = disk
	}
}

func WithConversionsDisk(disk string) Option {
	return func(o *Options) {
		o.ConversionsDisk = disk
	}
}

func WithDisk(disk string) Option {
	return WithDefaultDisk(disk)
}

func WithAutoGenerateConversions(enable bool) Option {
	return func(o *Options) {
		o.AutoGenerateConversions = enable
	}
}

func WithPerformConversions(conversions []string) Option {
	return func(o *Options) {
		o.PerformConversions = conversions
	}
}

func WithGenerateResponsiveImages(conversions []string) Option {
	return func(o *Options) {
		o.GenerateResponsiveImages = conversions
	}
}

func WithCustomProperties(properties map[string]interface{}) Option {
	return func(o *Options) {
		for k, v := range properties {
			o.CustomProperties[k] = v
		}
	}
}

func WithModel(modelType string, modelID uint64) Option {
	return func(o *Options) {
		o.ModelType = modelType
		o.ModelID = modelID
	}
}

func WithPathGeneratorPrefix(prefix string) Option {
	return func(o *Options) {
		o.PathGeneratorPrefix = prefix
	}
}

func WithName(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

func (m *DefaultMediaLibrary) AddMediaFromURL(
	ctx context.Context,
	urlStr string,
	collection string,
	options ...Option,
) (*models.Media, error) {

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	opts := &Options{
		DefaultDisk:              m.defaultOptions.DefaultDisk,
		ConversionsDisk:          m.defaultOptions.ConversionsDisk,
		AutoGenerateConversions:  m.defaultOptions.AutoGenerateConversions,
		PerformConversions:       m.defaultOptions.PerformConversions,
		GenerateResponsiveImages: m.defaultOptions.GenerateResponsiveImages,
		CustomProperties:         make(map[string]interface{}),
	}

	for k, v := range m.defaultOptions.CustomProperties {
		opts.CustomProperties[k] = v
	}

	for _, opt := range options {
		opt(opts)
	}

	// Set default name if not provided
	baseName := filepath.Base(parsedURL.Path)
	if opts.Name == "" {
		opts.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	diskName := opts.DefaultDisk
	disk, err := m.diskManager.GetDisk(diskName)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk %s: %w", diskName, err)
	}

	media := &models.Media{
		ModelType:            opts.ModelType,
		ModelID:              opts.ModelID,
		UUID:                 &id,
		CollectionName:       collection,
		Name:                 opts.Name,
		FileName:             baseName,
		Disk:                 diskName,
		ConversionsDisk:      opts.ConversionsDisk,
		Manipulations:        json.RawMessage("{}"),
		CustomProperties:     json.RawMessage("{}"),
		GeneratedConversions: json.RawMessage("{}"),
		ResponsiveImages:     json.RawMessage("{}"),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(opts.CustomProperties) > 0 {
		customPropsBytes, err := json.Marshal(opts.CustomProperties)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	path := m.pathGenerator.GetPath(media)

	err = disk.SaveFromURL(ctx, path, urlStr,
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to download and store file: %w", err)
	}

	exists, err := disk.Exists(ctx, path)
	if err != nil || !exists {
		return nil, fmt.Errorf("failed to verify file existence: %w", err)
	}

	fileReader, err := disk.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	fileBytes, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	media.Size = int64(len(fileBytes))

	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))

	err = m.repository.Save(ctx, media)
	if err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		err = m.PerformConversions(ctx, media, opts.PerformConversions...)
		if err != nil {

			fmt.Printf("Error performing conversions: %v\n", err)
		}
	}

	if len(opts.GenerateResponsiveImages) > 0 {
		err = m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...)
		if err != nil {

			fmt.Printf("Error generating responsive images: %v\n", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) PerformConversions(ctx context.Context, media *models.Media, conversionNames ...string) error {

	sourceDisk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		return fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	conversionsDisk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		return fmt.Errorf("failed to get conversions disk %s: %w", media.ConversionsDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	fileReader, err := sourceDisk.Get(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get original file: %w", err)
	}
	defer fileReader.Close()

	img, _, err := image.Decode(fileReader)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	generatedConversions := make(map[string]bool)

	if media.GeneratedConversions != nil && len(media.GeneratedConversions) > 0 {
		err = json.Unmarshal(media.GeneratedConversions, &generatedConversions)
		if err != nil {

			generatedConversions = make(map[string]bool)
		}
	}

	for _, conversionName := range conversionNames {

		if generatedConversions[conversionName] {
			continue
		}

		transformed, err := m.transformer.Transform(ctx, img, conversionName)
		if err != nil {

			fmt.Printf("Error transforming image for conversion %s: %v\n", conversionName, err)
			continue
		}

		conversionPath := m.pathGenerator.GetPathForConversion(media, conversionName)

		pr, pw := io.Pipe()
		go func() {

			var encodeErr error
			switch filepath.Ext(media.FileName) {
			case ".png":
				encodeErr = png.Encode(pw, transformed)
			case ".gif":
				encodeErr = gif.Encode(pw, transformed, nil)
			default:

				encodeErr = jpeg.Encode(pw, transformed, &jpeg.Options{Quality: 90})
			}

			if encodeErr != nil {
				pw.CloseWithError(encodeErr)
				return
			}
			pw.Close()
		}()

		err = conversionsDisk.Save(ctx, conversionPath, pr,
			storage.WithVisibility("public"),
			storage.WithContentType(media.MimeType))
		if err != nil {

			fmt.Printf("Error storing converted image for %s: %v\n", conversionName, err)
			continue
		}

		generatedConversions[conversionName] = true
	}

	generatedConversionsBytes, err := json.Marshal(generatedConversions)
	if err != nil {
		return fmt.Errorf("failed to marshal generated conversions: %w", err)
	}

	media.GeneratedConversions = generatedConversionsBytes
	media.UpdatedAt = time.Now()

	return m.repository.Save(ctx, media)
}

func (m *DefaultMediaLibrary) GenerateResponsiveImages(ctx context.Context, media *models.Media, conversionNames ...string) error {

	sourceDisk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		return fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	conversionsDisk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		return fmt.Errorf("failed to get conversions disk %s: %w", media.ConversionsDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	fileReader, err := sourceDisk.Get(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get original file: %w", err)
	}
	defer fileReader.Close()

	img, _, err := image.Decode(fileReader)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	responsiveImages := make(map[string]map[string]bool)

	if media.ResponsiveImages != nil && len(media.ResponsiveImages) > 0 {
		err = json.Unmarshal(media.ResponsiveImages, &responsiveImages)
		if err != nil {

			responsiveImages = make(map[string]map[string]bool)
		}
	}

	responsiveConversions := m.transformer.GetResponsiveImageConversions()

	for _, conversionName := range conversionNames {
		responsiveConversion, exists := responsiveConversions[conversionName]
		if !exists {

			continue
		}

		if responsiveImages[conversionName] == nil {
			responsiveImages[conversionName] = make(map[string]bool)
		}

		for _, width := range responsiveConversion.Widths {

			widthKey := fmt.Sprintf("%d", width)
			if responsiveImages[conversionName][widthKey] {
				continue
			}

			opts := responsiveConversion.Options
			opts.Width = width

			transformed, err := m.transformer.Transform(ctx, img, conversionName, conversion.WithWidth(width))
			if err != nil {

				fmt.Printf("Error generating responsive image for %s width %d: %v\n", conversionName, width, err)
				continue
			}

			responsivePath := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)

			pr, pw := io.Pipe()
			go func() {

				var encodeErr error
				switch filepath.Ext(media.FileName) {
				case ".png":
					encodeErr = png.Encode(pw, transformed)
				case ".gif":
					encodeErr = gif.Encode(pw, transformed, nil)
				default:

					encodeErr = jpeg.Encode(pw, transformed, &jpeg.Options{Quality: 90})
				}

				if encodeErr != nil {
					pw.CloseWithError(encodeErr)
					return
				}
				pw.Close()
			}()

			err = conversionsDisk.Save(ctx, responsivePath, pr,
				storage.WithVisibility("public"),
				storage.WithContentType(media.MimeType))
			if err != nil {

				fmt.Printf("Error storing responsive image for %s width %d: %v\n", conversionName, width, err)
				continue
			}

			responsiveImages[conversionName][widthKey] = true
		}
	}

	responsiveImagesBytes, err := json.Marshal(responsiveImages)
	if err != nil {
		return fmt.Errorf("failed to marshal responsive images: %w", err)
	}

	media.ResponsiveImages = responsiveImagesBytes
	media.UpdatedAt = time.Now()

	return m.repository.Save(ctx, media)
}

// GetURLForMedia returns the URL for accessing a media file
// Consider using GetMediaUrl which is the preferred naming convention
func (m *DefaultMediaLibrary) GetURLForMedia(media *models.Media) string {
	if media == nil {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.Disk, err)
		return ""
	}

	path := m.pathGenerator.GetPath(media)
	return disk.URL(path)
}

// GetURLForMediaConversion returns the URL for a specific conversion of a media file
func (m *DefaultMediaLibrary) GetURLForMediaConversion(media *models.Media, conversionName string) string {
	if media == nil {
		return ""
	}

	generatedConversions := make(map[string]bool)
	if err := json.Unmarshal(media.GeneratedConversions, &generatedConversions); err != nil {
		fmt.Printf("Error unmarshalling generated conversions: %v\n", err)
		return ""
	}

	if !generatedConversions[conversionName] {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForConversion(media, conversionName)
	return disk.URL(path)
}

// GetURLForResponsiveImage returns the URL for a responsive image with the specified width
func (m *DefaultMediaLibrary) GetURLForResponsiveImage(media *models.Media, conversionName string, width int) string {
	if media == nil {
		return ""
	}

	responsiveImages := make(map[string]map[string][]int)
	if err := json.Unmarshal(media.ResponsiveImages, &responsiveImages); err != nil {
		fmt.Printf("Error unmarshalling responsive images: %v\n", err)
		return ""
	}

	if _, ok := responsiveImages[conversionName]; !ok {
		return ""
	}

	if _, ok := responsiveImages[conversionName]["widths"]; !ok {
		return ""
	}

	widths := responsiveImages[conversionName]["widths"]
	found := false
	for _, w := range widths {
		if w == width {
			found = true
			break
		}
	}

	if !found {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)
	return disk.URL(path)
}

// GetMediaUrl is an alias for GetURLForMedia that follows a more consistent naming convention
// It returns the URL for accessing the media file using the path generator and disk configuration
func (m *DefaultMediaLibrary) GetMediaUrl(media *models.Media) string {
	if media == nil {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.Disk, err)
		return ""
	}

	path := m.pathGenerator.GetPath(media)
	return disk.URL(path)
}

// GetMediaConversionUrl is an alias for GetURLForMediaConversion that follows a more consistent naming convention
// It returns the URL for a specific conversion of a media file
func (m *DefaultMediaLibrary) GetMediaConversionUrl(media *models.Media, conversionName string) string {
	if media == nil {
		return ""
	}

	generatedConversions := make(map[string]bool)
	if err := json.Unmarshal(media.GeneratedConversions, &generatedConversions); err != nil {
		fmt.Printf("Error unmarshalling generated conversions: %v\n", err)
		return ""
	}

	if !generatedConversions[conversionName] {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForConversion(media, conversionName)
	return disk.URL(path)
}

// GetMediaResponsiveImageUrl is an alias for GetURLForResponsiveImage that follows a more consistent naming convention
// It returns the URL for a responsive image of the specified width
func (m *DefaultMediaLibrary) GetMediaResponsiveImageUrl(media *models.Media, conversionName string, width int) string {
	if media == nil {
		return ""
	}

	responsiveImages := make(map[string]map[string][]int)
	if err := json.Unmarshal(media.ResponsiveImages, &responsiveImages); err != nil {
		fmt.Printf("Error unmarshalling responsive images: %v\n", err)
		return ""
	}

	if _, ok := responsiveImages[conversionName]; !ok {
		return ""
	}

	if _, ok := responsiveImages[conversionName]["widths"]; !ok {
		return ""
	}

	widths := responsiveImages[conversionName]["widths"]
	found := false
	for _, w := range widths {
		if w == width {
			found = true
			break
		}
	}

	if !found {
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		fmt.Printf("Error getting disk %s: %v\n", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)
	return disk.URL(path)
}

func (m *DefaultMediaLibrary) GetMediaRepository() MediaRepository {
	return m.repository
}

func getMimeTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func (m *DefaultMediaLibrary) AddMediaFromURLToModel(
	ctx context.Context,
	urlStr string,
	modelType string,
	modelID uint64,
	collection string,
	options ...Option,
) (*models.Media, error) {

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	opts := &Options{
		DefaultDisk:              m.defaultOptions.DefaultDisk,
		ConversionsDisk:          m.defaultOptions.ConversionsDisk,
		AutoGenerateConversions:  m.defaultOptions.AutoGenerateConversions,
		PerformConversions:       m.defaultOptions.PerformConversions,
		GenerateResponsiveImages: m.defaultOptions.GenerateResponsiveImages,
		CustomProperties:         make(map[string]interface{}),
	}

	for k, v := range m.defaultOptions.CustomProperties {
		opts.CustomProperties[k] = v
	}

	// Include model info in options
	options = append([]Option{WithModel(modelType, modelID)}, options...)

	for _, opt := range options {
		opt(opts)
	}

	// Set default name if not provided
	baseName := filepath.Base(parsedURL.Path)
	if opts.Name == "" {
		opts.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	media := &models.Media{
		ModelType:            modelType,
		ModelID:              modelID,
		UUID:                 &id,
		CollectionName:       collection,
		Name:                 opts.Name,
		FileName:             baseName,
		Disk:                 opts.DefaultDisk,
		ConversionsDisk:      opts.ConversionsDisk,
		Manipulations:        json.RawMessage("{}"),
		CustomProperties:     json.RawMessage("{}"),
		GeneratedConversions: json.RawMessage("{}"),
		ResponsiveImages:     json.RawMessage("{}"),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(opts.CustomProperties) > 0 {
		customPropsBytes, err := json.Marshal(opts.CustomProperties)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk %s: %w", media.Disk, err)
	}

	path := m.pathGenerator.GetPath(media)

	err = disk.SaveFromURL(ctx, path, urlStr,
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to download and store file: %w", err)
	}

	exists, err := disk.Exists(ctx, path)
	if err != nil || !exists {
		return nil, fmt.Errorf("failed to verify file existence: %w", err)
	}

	fileReader, err := disk.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	fileBytes, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	media.Size = int64(len(fileBytes))

	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))

	err = m.repository.Save(ctx, media)
	if err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		err = m.PerformConversions(ctx, media, opts.PerformConversions...)
		if err != nil {

			fmt.Printf("Error performing conversions: %v\n", err)
		}
	}

	if len(opts.GenerateResponsiveImages) > 0 {
		err = m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...)
		if err != nil {

			fmt.Printf("Error generating responsive images: %v\n", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) GetMediaForModel(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error) {
	repo, ok := m.repository.(interface {
		FindByModelTypeAndID(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error)
	})

	if !ok {
		return nil, fmt.Errorf("repository does not support FindByModelTypeAndID")
	}

	return repo.FindByModelTypeAndID(ctx, modelType, modelID)
}

func (m *DefaultMediaLibrary) GetMediaForModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error) {
	repo, ok := m.repository.(interface {
		FindByModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error)
	})

	if !ok {
		return nil, fmt.Errorf("repository does not support FindByModelAndCollection")
	}

	return repo.FindByModelAndCollection(ctx, modelType, modelID, collection)
}

func (m *DefaultMediaLibrary) AddMediaFromDisk(
	ctx context.Context,
	filePath string,
	collection string,
	options ...Option,
) (*models.Media, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	opts := &Options{
		DefaultDisk:              m.defaultOptions.DefaultDisk,
		ConversionsDisk:          m.defaultOptions.ConversionsDisk,
		AutoGenerateConversions:  m.defaultOptions.AutoGenerateConversions,
		PerformConversions:       m.defaultOptions.PerformConversions,
		GenerateResponsiveImages: m.defaultOptions.GenerateResponsiveImages,
		CustomProperties:         make(map[string]interface{}),
	}

	for k, v := range m.defaultOptions.CustomProperties {
		opts.CustomProperties[k] = v
	}

	for _, opt := range options {
		opt(opts)
	}

	// Set default name if not provided
	baseName := filepath.Base(filePath)
	if opts.Name == "" {
		opts.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	diskName := opts.DefaultDisk
	disk, err := m.diskManager.GetDisk(diskName)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk %s: %w", diskName, err)
	}

	media := &models.Media{
		ModelType:            opts.ModelType,
		ModelID:              opts.ModelID,
		UUID:                 &id,
		CollectionName:       collection,
		Name:                 opts.Name,
		FileName:             baseName,
		Disk:                 diskName,
		ConversionsDisk:      opts.ConversionsDisk,
		Manipulations:        json.RawMessage("{}"),
		CustomProperties:     json.RawMessage("{}"),
		GeneratedConversions: json.RawMessage("{}"),
		ResponsiveImages:     json.RawMessage("{}"),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(opts.CustomProperties) > 0 {
		customPropsBytes, err := json.Marshal(opts.CustomProperties)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	path := m.pathGenerator.GetPath(media)

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	file.Seek(0, 0)

	err = disk.Save(ctx, path, file,
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	media.Size = int64(len(fileContent))
	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))

	if err := m.repository.Save(ctx, media); err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		if err := m.PerformConversions(ctx, media, opts.PerformConversions...); err != nil {

			fmt.Printf("Warning: Failed to perform conversions: %v\n", err)
		}
	}

	if opts.AutoGenerateConversions && len(opts.GenerateResponsiveImages) > 0 {
		if err := m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...); err != nil {

			fmt.Printf("Warning: Failed to generate responsive images: %v\n", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) AddMediaFromDiskToDisk(
	ctx context.Context,
	sourceDisk string,
	sourcePath string,
	targetDisk string,
	collection string,
	options ...Option,
) (*models.Media, error) {

	sourceDiskStorage, err := m.diskManager.GetDisk(sourceDisk)
	if err != nil {
		return nil, fmt.Errorf("failed to get source disk %s: %w", sourceDisk, err)
	}

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("file %s does not exist on disk %s", sourcePath, sourceDisk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	opts := &Options{
		DefaultDisk:              targetDisk,
		ConversionsDisk:          m.defaultOptions.ConversionsDisk,
		AutoGenerateConversions:  m.defaultOptions.AutoGenerateConversions,
		PerformConversions:       m.defaultOptions.PerformConversions,
		GenerateResponsiveImages: m.defaultOptions.GenerateResponsiveImages,
		CustomProperties:         make(map[string]interface{}),
	}

	for k, v := range m.defaultOptions.CustomProperties {
		opts.CustomProperties[k] = v
	}

	for _, opt := range options {
		opt(opts)
	}

	targetDiskStorage, err := m.diskManager.GetDisk(targetDisk)
	if err != nil {
		return nil, fmt.Errorf("failed to get target disk %s: %w", targetDisk, err)
	}

	// Set default name if not provided
	baseName := filepath.Base(sourcePath)
	if opts.Name == "" {
		opts.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	media := &models.Media{
		ModelType:            opts.ModelType,
		ModelID:              opts.ModelID,
		UUID:                 &id,
		CollectionName:       collection,
		Name:                 opts.Name,
		FileName:             baseName,
		Disk:                 targetDisk,
		ConversionsDisk:      opts.ConversionsDisk,
		Manipulations:        json.RawMessage("{}"),
		CustomProperties:     json.RawMessage("{}"),
		GeneratedConversions: json.RawMessage("{}"),
		ResponsiveImages:     json.RawMessage("{}"),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(opts.CustomProperties) > 0 {
		customPropsBytes, err := json.Marshal(opts.CustomProperties)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	media.Size = int64(len(fileContent))
	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))

	if err := m.repository.Save(ctx, media); err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	path := m.pathGenerator.GetPath(media)

	err = targetDiskStorage.Save(ctx, path, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		if err := m.PerformConversions(ctx, media, opts.PerformConversions...); err != nil {

			fmt.Printf("Warning: Failed to perform conversions: %v\n", err)
		}
	}

	if opts.AutoGenerateConversions && len(opts.GenerateResponsiveImages) > 0 {
		if err := m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...); err != nil {

			fmt.Printf("Warning: Failed to generate responsive images: %v\n", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) CopyMediaToDisk(
	ctx context.Context,
	media *models.Media,
	targetDisk string,
) (*models.Media, error) {

	sourceDiskStorage, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		return nil, fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	targetDiskStorage, err := m.diskManager.GetDisk(targetDisk)
	if err != nil {
		return nil, fmt.Errorf("failed to get target disk %s: %w", targetDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("file does not exist on disk %s", media.Disk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	copiedMedia := &models.Media{
		ModelType:            media.ModelType,
		ModelID:              media.ModelID,
		UUID:                 &id,
		CollectionName:       media.CollectionName,
		Name:                 media.Name,
		FileName:             media.FileName,
		MimeType:             media.MimeType,
		Disk:                 targetDisk,
		ConversionsDisk:      media.ConversionsDisk,
		Size:                 media.Size,
		Manipulations:        media.Manipulations,
		CustomProperties:     media.CustomProperties,
		GeneratedConversions: media.GeneratedConversions,
		ResponsiveImages:     media.ResponsiveImages,
		OrderColumn:          media.OrderColumn,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	targetPath := m.pathGenerator.GetPath(copiedMedia)

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	err = targetDiskStorage.Save(ctx, targetPath, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	copiedMedia.Size = int64(len(fileContent))
	copiedMedia.MimeType = getMimeTypeFromExtension(filepath.Ext(copiedMedia.FileName))

	if err := m.repository.Save(ctx, copiedMedia); err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	return copiedMedia, nil
}

func (m *DefaultMediaLibrary) MoveMediaToDisk(ctx context.Context, media *models.Media, targetDisk string) (*models.Media, error) {

	sourceDiskStorage, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		return nil, fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	targetDiskStorage, err := m.diskManager.GetDisk(targetDisk)
	if err != nil {
		return nil, fmt.Errorf("failed to get target disk %s: %w", targetDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("file does not exist on disk %s", media.Disk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	movedMedia := &models.Media{
		ModelType:            media.ModelType,
		ModelID:              media.ModelID,
		UUID:                 &id,
		CollectionName:       media.CollectionName,
		Name:                 media.Name,
		FileName:             media.FileName,
		MimeType:             media.MimeType,
		Disk:                 targetDisk,
		ConversionsDisk:      media.ConversionsDisk,
		Size:                 media.Size,
		Manipulations:        media.Manipulations,
		CustomProperties:     media.CustomProperties,
		GeneratedConversions: media.GeneratedConversions,
		ResponsiveImages:     media.ResponsiveImages,
		OrderColumn:          media.OrderColumn,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	targetPath := m.pathGenerator.GetPath(movedMedia)

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	err = targetDiskStorage.Save(ctx, targetPath, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	movedMedia.Size = int64(len(fileContent))
	movedMedia.MimeType = getMimeTypeFromExtension(filepath.Ext(movedMedia.FileName))

	if err := m.repository.Save(ctx, movedMedia); err != nil {
		return nil, fmt.Errorf("failed to save media: %w", err)
	}

	err = sourceDiskStorage.Delete(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	return movedMedia, nil
}
