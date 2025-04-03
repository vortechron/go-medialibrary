package conversion

import (
	"context"
	"image"
)


type Transformer interface {

	Transform(ctx context.Context, img image.Image, conversionName string, options ...Option) (image.Image, error)


	RegisterConversion(name string, conversion Conversion)


	RegisterResponsiveImageConversion(name string, widths []int, options ...Option)


	GetRegisteredConversions() map[string]Conversion


	GetResponsiveImageConversions() map[string]ResponsiveConversion


	ResizeImage(img image.Image, width, height int, opts *Options) (image.Image, error)
}


type Conversion func(img image.Image, opts *Options) (image.Image, error)


type ResponsiveConversion struct {
	Widths  []int
	Options *Options
}


type Option func(*Options)


type Options struct {
	Width       int
	Height      int
	Quality     int
	Format      string
	Fit         string
	Orientation string
	Background  string
	Border      string
	Blur        int
	Sharpen     int
	BrightnessQ int
	ContrastQ   int
	Watermark   string
}


func WithWidth(width int) Option {
	return func(o *Options) {
		o.Width = width
	}
}


func WithHeight(height int) Option {
	return func(o *Options) {
		o.Height = height
	}
}


func WithQuality(quality int) Option {
	return func(o *Options) {
		o.Quality = quality
	}
}


func WithFormat(format string) Option {
	return func(o *Options) {
		o.Format = format
	}
}


func WithFit(fit string) Option {
	return func(o *Options) {
		o.Fit = fit
	}
}


func WithOrientation(orientation string) Option {
	return func(o *Options) {
		o.Orientation = orientation
	}
}


func WithBackground(background string) Option {
	return func(o *Options) {
		o.Background = background
	}
}


func WithBorder(border string) Option {
	return func(o *Options) {
		o.Border = border
	}
}


func WithBlur(blur int) Option {
	return func(o *Options) {
		o.Blur = blur
	}
}


func WithSharpen(sharpen int) Option {
	return func(o *Options) {
		o.Sharpen = sharpen
	}
}


func WithBrightness(brightness int) Option {
	return func(o *Options) {
		o.BrightnessQ = brightness
	}
}


func WithContrast(contrast int) Option {
	return func(o *Options) {
		o.ContrastQ = contrast
	}
}


func WithWatermark(watermark string) Option {
	return func(o *Options) {
		o.Watermark = watermark
	}
}


func NewOptions(opts ...Option) *Options {
	options := &Options{
		Quality: 90,
		Format:  "jpg",
		Fit:     "contain",
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
