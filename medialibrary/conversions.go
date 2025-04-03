package medialibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"time"

	"github.com/vortechron/go-medialibrary/conversion"
	"github.com/vortechron/go-medialibrary/models"
	"github.com/vortechron/go-medialibrary/storage"
)

// PerformConversions performs the specified conversions on the media file
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

// GenerateResponsiveImages generates responsive images for the specified conversions
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
