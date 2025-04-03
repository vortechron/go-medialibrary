package medialibrary

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/vortechron/go-medialibrary/models"
)

// getMimeTypeFromContent detects the MIME type from file content
func getMimeTypeFromContent(content io.Reader) (string, error) {
	mime, err := mimetype.DetectReader(content)
	if err != nil {
		return "application/octet-stream", err
	}
	return mime.String(), nil
}

// getMimeTypeFromExtension returns the MIME type for a given file extension
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

// GetMediaRepository returns the media repository
func (m *DefaultMediaLibrary) GetMediaRepository() MediaRepository {
	return m.repository
}

// GetMediaForModel returns all media items for a given model
func (m *DefaultMediaLibrary) GetMediaForModel(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error) {
	repo, ok := m.repository.(interface {
		FindByModelTypeAndID(ctx context.Context, modelType string, modelID uint64) ([]*models.Media, error)
	})

	if !ok {
		return nil, fmt.Errorf("repository does not support FindByModelTypeAndID")
	}

	return repo.FindByModelTypeAndID(ctx, modelType, modelID)
}

// GetMediaForModelAndCollection returns all media items for a given model and collection
func (m *DefaultMediaLibrary) GetMediaForModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error) {
	repo, ok := m.repository.(interface {
		FindByModelAndCollection(ctx context.Context, modelType string, modelID uint64, collection string) ([]*models.Media, error)
	})

	if !ok {
		return nil, fmt.Errorf("repository does not support FindByModelAndCollection")
	}

	return repo.FindByModelAndCollection(ctx, modelType, modelID, collection)
}
