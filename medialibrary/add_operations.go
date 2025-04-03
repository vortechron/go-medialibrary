package medialibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vortechron/go-medialibrary/models"
	"github.com/vortechron/go-medialibrary/storage"
)

// AddMediaFromURL adds a media item from a URL
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

	// Detect MIME type from content
	contentReader := bytes.NewReader(fileBytes)
	mimeType, err := getMimeTypeFromContent(contentReader)
	if err != nil {
		m.logger.Warning("Failed to detect MIME type from content: %v, falling back to extension-based detection", err)
		mimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))
	}

	// Reset content reader for potential future use
	contentReader.Seek(0, 0)

	// Update the media with the actual file size and detected MIME type
	media.Size = int64(len(fileBytes))
	media.MimeType = mimeType
	m.logger.Debug("Detected mime type: %s for file size: %d bytes", media.MimeType, media.Size)
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

// AddMediaFromURLToModel adds a media item from a URL to a specific model
func (m *DefaultMediaLibrary) AddMediaFromURLToModel(
	ctx context.Context,
	urlStr string,
	modelType string,
	modelID uint64,
	collection string,
	options ...Option,
) (*models.Media, error) {
	// Include model info in options
	options = append([]Option{WithModel(modelType, modelID)}, options...)

	return m.AddMediaFromURL(ctx, urlStr, collection, options...)
}
