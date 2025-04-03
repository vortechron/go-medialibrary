package medialibrary

import (
	"context"

	"github.com/vortechron/go-medialibrary/models"
)

// MediaLibrary defines the interface for the media library functionality
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

// MediaRepository defines the interface for storage and retrieval of media records
type MediaRepository interface {
	Save(ctx context.Context, media *models.Media) error

	FindByID(ctx context.Context, id uint64) (*models.Media, error)

	Delete(ctx context.Context, media *models.Media) error
}

// PathGenerator defines the interface for generating file paths for media items
type PathGenerator interface {
	GetPath(media *models.Media) string

	GetPathForConversion(media *models.Media, conversionName string) string

	GetPathForResponsiveImage(media *models.Media, conversionName string, width int) string
}
