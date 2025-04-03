package medialibrary

import (
	"encoding/json"

	"github.com/vortechron/go-medialibrary/models"
)

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
