package medialibrary

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vortechron/go-medialibrary/models"
	"github.com/vortechron/go-medialibrary/storage"
)

// CopyMediaToDisk copies a media item to another disk
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

// MoveMediaToDisk moves a media item to another disk
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

	// Detect MIME type from content
	contentReader := bytes.NewReader(fileContent)
	mimeType, err := getMimeTypeFromContent(contentReader)
	if err != nil {
		m.logger.Warning("Failed to detect MIME type from content: %v, falling back to extension-based detection", err)
		mimeType = getMimeTypeFromExtension(filepath.Ext(media.FileName))
	}

	// Reset content reader for potential future use
	contentReader.Seek(0, 0)

	movedMedia := &models.Media{
		ModelType:            media.ModelType,
		ModelID:              media.ModelID,
		UUID:                 &id,
		CollectionName:       media.CollectionName,
		Name:                 media.Name,
		FileName:             media.FileName,
		MimeType:             mimeType,
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
