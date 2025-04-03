package medialibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vortechron/go-medialibrary/models"
	"github.com/vortechron/go-medialibrary/storage"
)

// AddMediaFromDisk adds a media item from a local file
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

	// Detect MIME type from content
	contentReader := bytes.NewReader(fileContent)
	mimeType, err := getMimeTypeFromContent(contentReader)
	if err != nil {
		m.logger.Warning("Failed to detect MIME type from content: %v, falling back to extension-based detection", err)
		mimeType = getMimeTypeFromExtension(filepath.Ext(baseName))
	}

	// Reset content reader for potential future use
	contentReader.Seek(0, 0)

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
		MimeType:             mimeType,
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

// AddMediaFromDiskToDisk adds a media item from one disk to another
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

	// Detect MIME type from content
	contentReader := bytes.NewReader(fileContent)
	mimeType, err := getMimeTypeFromContent(contentReader)
	if err != nil {
		m.logger.Warning("Failed to detect MIME type from content: %v, falling back to extension-based detection", err)
		mimeType = getMimeTypeFromExtension(filepath.Ext(baseName))
	}

	// Reset content reader for potential future use
	contentReader.Seek(0, 0)

	media.Size = int64(len(fileContent))
	media.MimeType = mimeType
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
