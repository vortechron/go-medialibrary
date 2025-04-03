package medialibrary

import (
	"github.com/vortechron/go-medialibrary/conversion"
	"github.com/vortechron/go-medialibrary/storage"
)

// DefaultMediaLibrary is the default implementation of the MediaLibrary interface
type DefaultMediaLibrary struct {
	diskManager    *storage.DiskManager
	transformer    conversion.Transformer
	repository     MediaRepository
	defaultOptions *Options
	pathGenerator  PathGenerator
	logger         Logger
}

// NewDefaultMediaLibrary creates a new default media library instance
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

// SetLogLevel sets the logging level for the media library
func (m *DefaultMediaLibrary) SetLogLevel(level LogLevel) {
	m.logger.SetLevel(level)
}

// GetLogger returns the logger for the media library
func (m *DefaultMediaLibrary) GetLogger() Logger {
	return m.logger
}
