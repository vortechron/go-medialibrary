package medialibrary

// Option is a function that configures Options
type Option func(*Options)

// Options holds the configuration for media operations
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

// WithDefaultDisk sets the default disk for media storage
func WithDefaultDisk(disk string) Option {
	return func(o *Options) {
		o.DefaultDisk = disk
	}
}

// WithConversionsDisk sets the disk for storing media conversions
func WithConversionsDisk(disk string) Option {
	return func(o *Options) {
		o.ConversionsDisk = disk
	}
}

// WithDisk is an alias for WithDefaultDisk
func WithDisk(disk string) Option {
	return WithDefaultDisk(disk)
}

// WithAutoGenerateConversions enables or disables automatic conversion generation
func WithAutoGenerateConversions(enable bool) Option {
	return func(o *Options) {
		o.AutoGenerateConversions = enable
	}
}

// WithPerformConversions specifies which conversions to perform
func WithPerformConversions(conversions []string) Option {
	return func(o *Options) {
		o.PerformConversions = conversions
	}
}

// WithGenerateResponsiveImages specifies which conversions should generate responsive images
func WithGenerateResponsiveImages(conversions []string) Option {
	return func(o *Options) {
		o.GenerateResponsiveImages = conversions
	}
}

// WithCustomProperties adds custom properties to the media
func WithCustomProperties(properties map[string]interface{}) Option {
	return func(o *Options) {
		for k, v := range properties {
			o.CustomProperties[k] = v
		}
	}
}

// WithModel sets the model type and ID for the media
func WithModel(modelType string, modelID uint64) Option {
	return func(o *Options) {
		o.ModelType = modelType
		o.ModelID = modelID
	}
}

// WithPathGeneratorPrefix sets the path prefix for the path generator
func WithPathGeneratorPrefix(prefix string) Option {
	return func(o *Options) {
		o.PathGeneratorPrefix = prefix
	}
}

// WithName sets the name for the media
func WithName(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

// WithLogLevel sets the log level for the media library
func WithLogLevel(level LogLevel) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}
