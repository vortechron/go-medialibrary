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
	"log"
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

// LogLevel defines the level of logging
type LogLevel int

const (
	// LogLevelNone means no logging
	LogLevelNone LogLevel = iota
	// LogLevelError logs only errors
	LogLevelError
	// LogLevelWarning logs warnings and errors
	LogLevelWarning
	// LogLevelInfo logs info, warnings, and errors
	LogLevelInfo
	// LogLevelDebug logs everything
	LogLevelDebug
)

// Logger defines the interface for logging
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warning(format string, args ...interface{})
	Error(format string, args ...interface{})
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// DefaultLogger implements the Logger interface
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewDefaultLogger creates a new default logger with the specified log level
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		logger: log.New(os.Stdout, "MediaLibrary: ", log.LstdFlags),
	}
}

// Debug logs debug messages
func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs informational messages
func (l *DefaultLogger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Warning logs warning messages
func (l *DefaultLogger) Warning(format string, args ...interface{}) {
	if l.level >= LogLevelWarning {
		l.logger.Printf("[WARNING] "+format, args...)
	}
}

// Error logs error messages
func (l *DefaultLogger) Error(format string, args ...interface{}) {
	if l.level >= LogLevelError {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// SetLevel sets the logging level
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current logging level
func (l *DefaultLogger) GetLevel() LogLevel {
	return l.level
}

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

	SetLogLevel(level LogLevel)

	GetLogger() Logger
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
	LogLevel                 LogLevel
}

type DefaultMediaLibrary struct {
	diskManager    *storage.DiskManager
	transformer    conversion.Transformer
	repository     MediaRepository
	defaultOptions *Options
	pathGenerator  PathGenerator
	logger         Logger
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

func (p *DefaultPathGenerator) cleanPath(path string) string {
	return filepath.Clean(path)
}

func (p *DefaultPathGenerator) GetPath(media *models.Media) string {
	return p.cleanPath(fmt.Sprintf("%s/%s",
		p.getBasePath(media),
		media.FileName))
}

func (p *DefaultPathGenerator) GetPathForConversion(media *models.Media, conversionName string) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return p.cleanPath(fmt.Sprintf("%s/%s/conversions/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+ext))
}

func (p *DefaultPathGenerator) GetPathForResponsiveImage(media *models.Media, conversionName string, width int) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return p.cleanPath(fmt.Sprintf("%s/%s/responsive-images/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+"-"+fmt.Sprintf("%d", width)+ext))
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
		LogLevel:         LogLevelWarning, // Default to warning level
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
		logger: NewDefaultLogger(opts.LogLevel),
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

func WithLogLevel(level LogLevel) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}

func (m *DefaultMediaLibrary) SetLogLevel(level LogLevel) {
	m.logger.SetLevel(level)
}

func (m *DefaultMediaLibrary) GetLogger() Logger {
	return m.logger
}

func (m *DefaultMediaLibrary) AddMediaFromURL(
	ctx context.Context,
	urlStr string,
	collection string,
	options ...Option,
) (*models.Media, error) {
	m.logger.Debug("Adding media from URL: %s to collection: %s", urlStr, collection)

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		m.logger.Error("Invalid URL: %v", err)
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
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
	m.logger.Debug("Using disk: %s", diskName)

	disk, err := m.diskManager.GetDisk(diskName)
	if err != nil {
		m.logger.Error("Failed to get disk %s: %v", diskName, err)
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
			m.logger.Error("Failed to marshal custom properties: %v", err)
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	// Set a dummy size and mime type initially
	media.Size = 0
	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))
	m.logger.Debug("Detected mime type: %s", media.MimeType)

	// Save to DB first to get the ID
	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to save media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Initially saved media with ID %d", media.ID)

	// Now we have the ID, we can generate the proper path
	path := m.pathGenerator.GetPath(media)
	m.logger.Info("Saving media from URL %s to path %s", urlStr, path)

	err = disk.SaveFromURL(ctx, path, urlStr,
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to download and store file: %v", err)
		return nil, fmt.Errorf("failed to download and store file: %w", err)
	}

	exists, err := disk.Exists(ctx, path)
	if err != nil || !exists {
		m.logger.Error("Failed to verify file existence: %v", err)
		return nil, fmt.Errorf("failed to verify file existence: %w", err)
	}

	fileReader, err := disk.Get(ctx, path)
	if err != nil {
		m.logger.Error("Failed to get file: %v", err)
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	fileBytes, err := ioutil.ReadAll(fileReader)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Update the media with the actual file size
	media.Size = int64(len(fileBytes))
	media.UpdatedAt = time.Now()

	// Save the updated media with the correct file size
	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to update media: %v", err)
		return nil, fmt.Errorf("failed to update media: %w", err)
	}
	m.logger.Info("Successfully updated media ID %d with file size", media.ID)

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		m.logger.Info("Performing %d conversions", len(opts.PerformConversions))
		err = m.PerformConversions(ctx, media, opts.PerformConversions...)
		if err != nil {
			m.logger.Warning("Error performing conversions: %v", err)
		}
	}

	if len(opts.GenerateResponsiveImages) > 0 {
		m.logger.Info("Generating responsive images for %d conversions", len(opts.GenerateResponsiveImages))
		err = m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...)
		if err != nil {
			m.logger.Warning("Error generating responsive images: %v", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) PerformConversions(ctx context.Context, media *models.Media, conversionNames ...string) error {
	m.logger.Info("Performing conversions for media ID %d: %v", media.ID, conversionNames)

	sourceDisk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Failed to get source disk %s: %v", media.Disk, err)
		return fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	conversionsDisk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Failed to get conversions disk %s: %v", media.ConversionsDisk, err)
		return fmt.Errorf("failed to get conversions disk %s: %w", media.ConversionsDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	m.logger.Debug("Reading source file from path: %s", sourcePath)

	fileReader, err := sourceDisk.Get(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to get original file: %v", err)
		return fmt.Errorf("failed to get original file: %w", err)
	}
	defer fileReader.Close()

	img, _, err := image.Decode(fileReader)
	if err != nil {
		m.logger.Error("Failed to decode image: %v", err)
		return fmt.Errorf("failed to decode image: %w", err)
	}

	generatedConversions := make(map[string]bool)

	if media.GeneratedConversions != nil && len(media.GeneratedConversions) > 0 {
		err = json.Unmarshal(media.GeneratedConversions, &generatedConversions)
		if err != nil {
			m.logger.Warning("Failed to unmarshal generated conversions, starting fresh: %v", err)
			generatedConversions = make(map[string]bool)
		}
	}

	for _, conversionName := range conversionNames {
		m.logger.Debug("Processing conversion: %s", conversionName)

		if generatedConversions[conversionName] {
			m.logger.Debug("Conversion %s already exists, skipping", conversionName)
			continue
		}

		transformed, err := m.transformer.Transform(ctx, img, conversionName)
		if err != nil {
			m.logger.Warning("Error transforming image for conversion %s: %v", conversionName, err)
			continue
		}

		conversionPath := m.pathGenerator.GetPathForConversion(media, conversionName)
		m.logger.Debug("Saving conversion to path: %s", conversionPath)

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
			m.logger.Warning("Error storing converted image for %s: %v", conversionName, err)
			continue
		}

		generatedConversions[conversionName] = true
		m.logger.Info("Successfully generated conversion: %s", conversionName)
	}

	generatedConversionsBytes, err := json.Marshal(generatedConversions)
	if err != nil {
		m.logger.Error("Failed to marshal generated conversions: %v", err)
		return fmt.Errorf("failed to marshal generated conversions: %w", err)
	}

	media.GeneratedConversions = generatedConversionsBytes
	media.UpdatedAt = time.Now()

	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to save media with updated conversions: %v", err)
		return fmt.Errorf("failed to save media: %w", err)
	}

	m.logger.Info("Completed performing conversions for media ID %d", media.ID)
	return nil
}

func (m *DefaultMediaLibrary) GenerateResponsiveImages(ctx context.Context, media *models.Media, conversionNames ...string) error {
	m.logger.Info("Generating responsive images for media ID %d: %v", media.ID, conversionNames)

	sourceDisk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Failed to get source disk %s: %v", media.Disk, err)
		return fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	conversionsDisk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Failed to get conversions disk %s: %v", media.ConversionsDisk, err)
		return fmt.Errorf("failed to get conversions disk %s: %w", media.ConversionsDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	m.logger.Debug("Reading source file from path: %s", sourcePath)

	fileReader, err := sourceDisk.Get(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to get original file: %v", err)
		return fmt.Errorf("failed to get original file: %w", err)
	}
	defer fileReader.Close()

	img, _, err := image.Decode(fileReader)
	if err != nil {
		m.logger.Error("Failed to decode image: %v", err)
		return fmt.Errorf("failed to decode image: %w", err)
	}

	responsiveImages := make(map[string]map[string]bool)

	if media.ResponsiveImages != nil && len(media.ResponsiveImages) > 0 {
		err = json.Unmarshal(media.ResponsiveImages, &responsiveImages)
		if err != nil {
			m.logger.Warning("Failed to unmarshal responsive images, starting fresh: %v", err)
			responsiveImages = make(map[string]map[string]bool)
		}
	}

	responsiveConversions := m.transformer.GetResponsiveImageConversions()
	m.logger.Debug("Available responsive conversions: %v", getMapKeys(responsiveConversions))

	for _, conversionName := range conversionNames {
		responsiveConversion, exists := responsiveConversions[conversionName]
		if !exists {
			m.logger.Warning("Responsive conversion %s not found in transformer", conversionName)
			continue
		}

		m.logger.Debug("Processing responsive images for conversion: %s", conversionName)

		if responsiveImages[conversionName] == nil {
			responsiveImages[conversionName] = make(map[string]bool)
		}

		for _, width := range responsiveConversion.Widths {
			widthKey := fmt.Sprintf("%d", width)
			if responsiveImages[conversionName][widthKey] {
				m.logger.Debug("Responsive image for %s at width %d already exists, skipping", conversionName, width)
				continue
			}

			m.logger.Debug("Generating responsive image for %s at width %d", conversionName, width)

			opts := responsiveConversion.Options
			opts.Width = width

			transformed, err := m.transformer.Transform(ctx, img, conversionName, conversion.WithWidth(width))
			if err != nil {
				m.logger.Warning("Error generating responsive image for %s width %d: %v", conversionName, width, err)
				continue
			}

			responsivePath := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)
			m.logger.Debug("Saving responsive image to path: %s", responsivePath)

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
				m.logger.Warning("Error storing responsive image for %s width %d: %v", conversionName, width, err)
				continue
			}

			responsiveImages[conversionName][widthKey] = true
			m.logger.Info("Successfully generated responsive image: %s at width %d", conversionName, width)
		}
	}

	responsiveImagesBytes, err := json.Marshal(responsiveImages)
	if err != nil {
		m.logger.Error("Failed to marshal responsive images: %v", err)
		return fmt.Errorf("failed to marshal responsive images: %w", err)
	}

	media.ResponsiveImages = responsiveImagesBytes
	media.UpdatedAt = time.Now()

	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to save media with updated responsive images: %v", err)
		return fmt.Errorf("failed to save media: %w", err)
	}

	m.logger.Info("Completed generating responsive images for media ID %d", media.ID)
	return nil
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]conversion.ResponsiveConversion) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetURLForMedia returns the URL for accessing a media file
// Consider using GetMediaUrl which is the preferred naming convention
func (m *DefaultMediaLibrary) GetURLForMedia(media *models.Media) string {
	if media == nil {
		m.logger.Debug("GetURLForMedia called with nil media")
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.Disk, err)
		return ""
	}

	path := m.pathGenerator.GetPath(media)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d: %s", media.ID, url)
	return url
}

// GetURLForMediaConversion returns the URL for a specific conversion of a media file
func (m *DefaultMediaLibrary) GetURLForMediaConversion(media *models.Media, conversionName string) string {
	if media == nil {
		m.logger.Debug("GetURLForMediaConversion called with nil media")
		return ""
	}

	generatedConversions := make(map[string]bool)
	if err := json.Unmarshal(media.GeneratedConversions, &generatedConversions); err != nil {
		m.logger.Error("Error unmarshalling generated conversions: %v", err)
		return ""
	}

	if !generatedConversions[conversionName] {
		m.logger.Debug("Conversion %s not found for media ID %d", conversionName, media.ID)
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForConversion(media, conversionName)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d conversion %s: %s", media.ID, conversionName, url)
	return url
}

// GetURLForResponsiveImage returns the URL for a responsive image with the specified width
func (m *DefaultMediaLibrary) GetURLForResponsiveImage(media *models.Media, conversionName string, width int) string {
	if media == nil {
		m.logger.Debug("GetURLForResponsiveImage called with nil media")
		return ""
	}

	responsiveImages := make(map[string]map[string][]int)
	if err := json.Unmarshal(media.ResponsiveImages, &responsiveImages); err != nil {
		m.logger.Error("Error unmarshalling responsive images: %v", err)
		return ""
	}

	if _, ok := responsiveImages[conversionName]; !ok {
		m.logger.Debug("Responsive conversion %s not found for media ID %d", conversionName, media.ID)
		return ""
	}

	if _, ok := responsiveImages[conversionName]["widths"]; !ok {
		m.logger.Debug("No widths found for conversion %s media ID %d", conversionName, media.ID)
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
		m.logger.Debug("Width %d not found for conversion %s media ID %d", width, conversionName, media.ID)
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d responsive image %s width %d: %s", media.ID, conversionName, width, url)
	return url
}

// GetMediaUrl is an alias for GetURLForMedia that follows a more consistent naming convention
// It returns the URL for accessing the media file using the path generator and disk configuration
func (m *DefaultMediaLibrary) GetMediaUrl(media *models.Media) string {
	if media == nil {
		m.logger.Debug("GetMediaUrl called with nil media")
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.Disk, err)
		return ""
	}

	path := m.pathGenerator.GetPath(media)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d: %s", media.ID, url)
	return url
}

// GetMediaConversionUrl is an alias for GetURLForMediaConversion that follows a more consistent naming convention
// It returns the URL for a specific conversion of a media file
func (m *DefaultMediaLibrary) GetMediaConversionUrl(media *models.Media, conversionName string) string {
	if media == nil {
		m.logger.Debug("GetMediaConversionUrl called with nil media")
		return ""
	}

	generatedConversions := make(map[string]bool)
	if err := json.Unmarshal(media.GeneratedConversions, &generatedConversions); err != nil {
		m.logger.Error("Error unmarshalling generated conversions: %v", err)
		return ""
	}

	if !generatedConversions[conversionName] {
		m.logger.Debug("Conversion %s not found for media ID %d", conversionName, media.ID)
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForConversion(media, conversionName)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d conversion %s: %s", media.ID, conversionName, url)
	return url
}

// GetMediaResponsiveImageUrl is an alias for GetURLForResponsiveImage that follows a more consistent naming convention
// It returns the URL for a responsive image of the specified width
func (m *DefaultMediaLibrary) GetMediaResponsiveImageUrl(media *models.Media, conversionName string, width int) string {
	if media == nil {
		m.logger.Debug("GetMediaResponsiveImageUrl called with nil media")
		return ""
	}

	responsiveImages := make(map[string]map[string][]int)
	if err := json.Unmarshal(media.ResponsiveImages, &responsiveImages); err != nil {
		m.logger.Error("Error unmarshalling responsive images: %v", err)
		return ""
	}

	if _, ok := responsiveImages[conversionName]; !ok {
		m.logger.Debug("Responsive conversion %s not found for media ID %d", conversionName, media.ID)
		return ""
	}

	if _, ok := responsiveImages[conversionName]["widths"]; !ok {
		m.logger.Debug("No widths found for conversion %s media ID %d", conversionName, media.ID)
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
		m.logger.Debug("Width %d not found for conversion %s media ID %d", width, conversionName, media.ID)
		return ""
	}

	disk, err := m.diskManager.GetDisk(media.ConversionsDisk)
	if err != nil {
		m.logger.Error("Error getting disk %s: %v", media.ConversionsDisk, err)
		return ""
	}

	path := m.pathGenerator.GetPathForResponsiveImage(media, conversionName, width)
	url := disk.URL(path)
	m.logger.Debug("Generated URL for media ID %d responsive image %s width %d: %s", media.ID, conversionName, width, url)
	return url
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
	m.logger.Debug("Adding media from URL: %s to model: %s ID: %d collection: %s", urlStr, modelType, modelID, collection)

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		m.logger.Error("Invalid URL: %v", err)
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
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
			m.logger.Error("Failed to marshal custom properties: %v", err)
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	// Set a dummy size and mime type initially
	media.Size = 0
	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))
	m.logger.Debug("Detected mime type: %s", media.MimeType)

	// Save to DB first to get the ID
	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to save media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Initially saved media with ID %d", media.ID)

	disk, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Failed to get disk %s: %v", media.Disk, err)
		return nil, fmt.Errorf("failed to get disk %s: %w", media.Disk, err)
	}

	// Now we have the ID, we can generate the proper path
	path := m.pathGenerator.GetPath(media)
	m.logger.Info("Saving media from URL %s to path %s", urlStr, path)

	err = disk.SaveFromURL(ctx, path, urlStr,
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to download and store file: %v", err)
		return nil, fmt.Errorf("failed to download and store file: %w", err)
	}

	exists, err := disk.Exists(ctx, path)
	if err != nil || !exists {
		m.logger.Error("Failed to verify file existence: %v", err)
		return nil, fmt.Errorf("failed to verify file existence: %w", err)
	}

	fileReader, err := disk.Get(ctx, path)
	if err != nil {
		m.logger.Error("Failed to get file: %v", err)
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	fileBytes, err := ioutil.ReadAll(fileReader)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Update media with actual file size
	media.Size = int64(len(fileBytes))
	media.UpdatedAt = time.Now()

	// Save the updated media with the correct file size
	err = m.repository.Save(ctx, media)
	if err != nil {
		m.logger.Error("Failed to update media: %v", err)
		return nil, fmt.Errorf("failed to update media: %w", err)
	}
	m.logger.Info("Successfully updated media ID %d with file size", media.ID)

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		m.logger.Info("Performing %d conversions", len(opts.PerformConversions))
		err = m.PerformConversions(ctx, media, opts.PerformConversions...)
		if err != nil {
			m.logger.Warning("Error performing conversions: %v", err)
		}
	}

	if len(opts.GenerateResponsiveImages) > 0 {
		m.logger.Info("Generating responsive images for %d conversions", len(opts.GenerateResponsiveImages))
		err = m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...)
		if err != nil {
			m.logger.Warning("Error generating responsive images: %v", err)
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
	m.logger.Debug("Adding media from disk path: %s to collection: %s", filePath, collection)

	file, err := os.Open(filePath)
	if err != nil {
		m.logger.Error("Failed to open file: %v", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
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
	m.logger.Debug("Using disk: %s", diskName)

	disk, err := m.diskManager.GetDisk(diskName)
	if err != nil {
		m.logger.Error("Failed to get disk %s: %v", diskName, err)
		return nil, fmt.Errorf("failed to get disk %s: %w", diskName, err)
	}

	// Read file content first to get size
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
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
		Size:                 int64(len(fileContent)),
		MimeType:             getMimeTypeFromExtension(filepath.Ext(baseName)),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if len(opts.CustomProperties) > 0 {
		customPropsBytes, err := json.Marshal(opts.CustomProperties)
		if err != nil {
			m.logger.Error("Failed to marshal custom properties: %v", err)
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	m.logger.Debug("Detected mime type: %s for file size: %d bytes", media.MimeType, media.Size)

	// Save to DB first to get the ID
	if err := m.repository.Save(ctx, media); err != nil {
		m.logger.Error("Failed to save media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Successfully saved media ID %d", media.ID)

	// Now we have the ID, we can generate the proper path
	path := m.pathGenerator.GetPath(media)
	m.logger.Info("Saving media from disk path %s to storage path %s", filePath, path)

	// Reset file pointer to beginning
	file.Seek(0, 0)

	// Save the file to disk
	err = disk.Save(ctx, path, file,
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to store file: %v", err)
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		m.logger.Info("Performing %d conversions", len(opts.PerformConversions))
		if err := m.PerformConversions(ctx, media, opts.PerformConversions...); err != nil {
			m.logger.Warning("Failed to perform conversions: %v", err)
		}
	}

	if opts.AutoGenerateConversions && len(opts.GenerateResponsiveImages) > 0 {
		m.logger.Info("Generating responsive images for %d conversions", len(opts.GenerateResponsiveImages))
		if err := m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...); err != nil {
			m.logger.Warning("Failed to generate responsive images: %v", err)
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
	m.logger.Debug("Adding media from disk %s path %s to disk %s collection %s", sourceDisk, sourcePath, targetDisk, collection)

	sourceDiskStorage, err := m.diskManager.GetDisk(sourceDisk)
	if err != nil {
		m.logger.Error("Failed to get source disk %s: %v", sourceDisk, err)
		return nil, fmt.Errorf("failed to get source disk %s: %w", sourceDisk, err)
	}

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to check if file exists: %v", err)
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		m.logger.Error("File %s does not exist on disk %s", sourcePath, sourceDisk)
		return nil, fmt.Errorf("file %s does not exist on disk %s", sourcePath, sourceDisk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to get file: %v", err)
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
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
		m.logger.Error("Failed to get target disk %s: %v", targetDisk, err)
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
			m.logger.Error("Failed to marshal custom properties: %v", err)
			return nil, fmt.Errorf("failed to marshal custom properties: %w", err)
		}
		media.CustomProperties = customPropsBytes
	}

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	media.Size = int64(len(fileContent))
	media.MimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))
	m.logger.Debug("Detected mime type: %s for file size: %d bytes", media.MimeType, media.Size)

	if err := m.repository.Save(ctx, media); err != nil {
		m.logger.Error("Failed to save media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Successfully saved media ID %d", media.ID)

	path := m.pathGenerator.GetPath(media)
	m.logger.Info("Saving media to target disk path: %s", path)

	err = targetDiskStorage.Save(ctx, path, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to store file: %v", err)
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	if opts.AutoGenerateConversions && len(opts.PerformConversions) > 0 {
		m.logger.Info("Performing %d conversions", len(opts.PerformConversions))
		if err := m.PerformConversions(ctx, media, opts.PerformConversions...); err != nil {
			m.logger.Warning("Failed to perform conversions: %v", err)
		}
	}

	if opts.AutoGenerateConversions && len(opts.GenerateResponsiveImages) > 0 {
		m.logger.Info("Generating responsive images for %d conversions", len(opts.GenerateResponsiveImages))
		if err := m.GenerateResponsiveImages(ctx, media, opts.GenerateResponsiveImages...); err != nil {
			m.logger.Warning("Failed to generate responsive images: %v", err)
		}
	}

	return media, nil
}

func (m *DefaultMediaLibrary) CopyMediaToDisk(
	ctx context.Context,
	media *models.Media,
	targetDisk string,
) (*models.Media, error) {
	m.logger.Debug("Copying media ID %d from disk %s to disk %s", media.ID, media.Disk, targetDisk)

	sourceDiskStorage, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Failed to get source disk %s: %v", media.Disk, err)
		return nil, fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	targetDiskStorage, err := m.diskManager.GetDisk(targetDisk)
	if err != nil {
		m.logger.Error("Failed to get target disk %s: %v", targetDisk, err)
		return nil, fmt.Errorf("failed to get target disk %s: %w", targetDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	m.logger.Debug("Source path: %s", sourcePath)

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to check if file exists: %v", err)
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		m.logger.Error("File does not exist on disk %s", media.Disk)
		return nil, fmt.Errorf("file does not exist on disk %s", media.Disk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to get file: %v", err)
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
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
		Size:                 int64(len(fileContent)),
		Manipulations:        media.Manipulations,
		CustomProperties:     media.CustomProperties,
		GeneratedConversions: media.GeneratedConversions,
		ResponsiveImages:     media.ResponsiveImages,
		OrderColumn:          media.OrderColumn,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Save to database first to get ID
	if err := m.repository.Save(ctx, copiedMedia); err != nil {
		m.logger.Error("Failed to save copied media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Successfully saved copied media ID %d", copiedMedia.ID)

	// Now we have the ID, get the proper path
	targetPath := m.pathGenerator.GetPath(copiedMedia)
	m.logger.Info("Copying media to target path: %s", targetPath)

	// Save to disk
	err = targetDiskStorage.Save(ctx, targetPath, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to store file: %v", err)
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	m.logger.Debug("Copied media with mime type: %s size: %d bytes", copiedMedia.MimeType, copiedMedia.Size)

	return copiedMedia, nil
}

func (m *DefaultMediaLibrary) MoveMediaToDisk(ctx context.Context, media *models.Media, targetDisk string) (*models.Media, error) {
	m.logger.Debug("Moving media ID %d from disk %s to disk %s", media.ID, media.Disk, targetDisk)

	sourceDiskStorage, err := m.diskManager.GetDisk(media.Disk)
	if err != nil {
		m.logger.Error("Failed to get source disk %s: %v", media.Disk, err)
		return nil, fmt.Errorf("failed to get source disk %s: %w", media.Disk, err)
	}

	targetDiskStorage, err := m.diskManager.GetDisk(targetDisk)
	if err != nil {
		m.logger.Error("Failed to get target disk %s: %v", targetDisk, err)
		return nil, fmt.Errorf("failed to get target disk %s: %w", targetDisk, err)
	}

	sourcePath := m.pathGenerator.GetPath(media)
	m.logger.Debug("Source path: %s", sourcePath)

	exists, err := sourceDiskStorage.Exists(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to check if file exists: %v", err)
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}
	if !exists {
		m.logger.Error("File does not exist on disk %s", media.Disk)
		return nil, fmt.Errorf("file does not exist on disk %s", media.Disk)
	}

	fileReader, err := sourceDiskStorage.Get(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to get file: %v", err)
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer fileReader.Close()

	id, err := uuid.NewV4()
	if err != nil {
		m.logger.Error("Failed to generate UUID: %v", err)
		return nil, fmt.Errorf("failed to generate uuid: %w", err)
	}

	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		m.logger.Error("Failed to read file: %v", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	movedMedia := &models.Media{
		ModelType:            media.ModelType,
		ModelID:              media.ModelID,
		UUID:                 &id,
		CollectionName:       media.CollectionName,
		Name:                 media.Name,
		FileName:             media.FileName,
		MimeType:             getMimeTypeFromExtension(filepath.Ext(media.FileName)),
		Disk:                 targetDisk,
		ConversionsDisk:      media.ConversionsDisk,
		Size:                 int64(len(fileContent)),
		Manipulations:        media.Manipulations,
		CustomProperties:     media.CustomProperties,
		GeneratedConversions: media.GeneratedConversions,
		ResponsiveImages:     media.ResponsiveImages,
		OrderColumn:          media.OrderColumn,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Save to database first to get ID
	if err := m.repository.Save(ctx, movedMedia); err != nil {
		m.logger.Error("Failed to save moved media: %v", err)
		return nil, fmt.Errorf("failed to save media: %w", err)
	}
	m.logger.Info("Successfully saved moved media ID %d", movedMedia.ID)

	// Now we have the ID, get the proper path
	targetPath := m.pathGenerator.GetPath(movedMedia)
	m.logger.Info("Moving media to target path: %s", targetPath)

	// Save to the target disk
	err = targetDiskStorage.Save(ctx, targetPath, strings.NewReader(string(fileContent)),
		storage.WithVisibility("public"))
	if err != nil {
		m.logger.Error("Failed to store file: %v", err)
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	m.logger.Debug("Moved media with mime type: %s size: %d bytes", movedMedia.MimeType, movedMedia.Size)

	// Delete from the source disk
	err = sourceDiskStorage.Delete(ctx, sourcePath)
	if err != nil {
		m.logger.Error("Failed to delete original file: %v", err)
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}
	m.logger.Info("Successfully deleted original file from disk %s path %s", media.Disk, sourcePath)

	return movedMedia, nil
}
