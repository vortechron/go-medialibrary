package medialibrary

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vortechron/go-medialibrary/models"
)

// DefaultPathGenerator implements the PathGenerator interface
type DefaultPathGenerator struct {
	prefix string
}

// getBasePath returns the base path for a media item
func (p *DefaultPathGenerator) getBasePath(media *models.Media) string {
	return fmt.Sprintf("%s/%d/", p.prefix, media.ID)
}

// cleanPath cleans up a file path
func (p *DefaultPathGenerator) cleanPath(path string) string {
	return filepath.Clean(path)
}

// GetPath returns the path for the original media file
func (p *DefaultPathGenerator) GetPath(media *models.Media) string {
	return p.cleanPath(fmt.Sprintf("%s/%s",
		p.getBasePath(media),
		media.FileName))
}

// GetPathForConversion returns the path for a media conversion
func (p *DefaultPathGenerator) GetPathForConversion(media *models.Media, conversionName string) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return p.cleanPath(fmt.Sprintf("%s/%s/conversions/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+ext))
}

// GetPathForResponsiveImage returns the path for a responsive image
func (p *DefaultPathGenerator) GetPathForResponsiveImage(media *models.Media, conversionName string, width int) string {
	ext := filepath.Ext(media.FileName)
	basename := strings.TrimSuffix(media.FileName, ext)

	return p.cleanPath(fmt.Sprintf("%s/%s/responsive-images/%s",
		p.getBasePath(media),
		conversionName,
		basename+"-"+conversionName+"-"+fmt.Sprintf("%d", width)+ext))
}
