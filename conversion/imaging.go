package conversion

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/disintegration/imaging"
)


type ImagingTransformer struct {
	conversions           map[string]Conversion
	responsiveConversions map[string]ResponsiveConversion
	mu                    sync.RWMutex
}


func NewImagingTransformer() *ImagingTransformer {
	return &ImagingTransformer{
		conversions:           make(map[string]Conversion),
		responsiveConversions: make(map[string]ResponsiveConversion),
	}
}


func (t *ImagingTransformer) RegisterConversion(name string, conversion Conversion) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.conversions[name] = conversion
}


func (t *ImagingTransformer) RegisterResponsiveImageConversion(name string, widths []int, options ...Option) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.responsiveConversions[name] = ResponsiveConversion{
		Widths:  widths,
		Options: NewOptions(options...),
	}
}


func (t *ImagingTransformer) GetRegisteredConversions() map[string]Conversion {
	t.mu.RLock()
	defer t.mu.RUnlock()


	result := make(map[string]Conversion, len(t.conversions))
	for k, v := range t.conversions {
		result[k] = v
	}

	return result
}


func (t *ImagingTransformer) GetResponsiveImageConversions() map[string]ResponsiveConversion {
	t.mu.RLock()
	defer t.mu.RUnlock()


	result := make(map[string]ResponsiveConversion, len(t.responsiveConversions))
	for k, v := range t.responsiveConversions {
		result[k] = v
	}

	return result
}


func (t *ImagingTransformer) Transform(ctx context.Context, img image.Image, conversionName string, options ...Option) (image.Image, error) {
	t.mu.RLock()
	conversion, exists := t.conversions[conversionName]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("conversion not found: %s", conversionName)
	}

	opts := NewOptions(options...)


	return conversion(img, opts)
}


func (t *ImagingTransformer) DefaultConversions() {

	t.RegisterConversion("thumbnail", func(img image.Image, opts *Options) (image.Image, error) {
		return t.ResizeImage(img, 150, 150, opts)
	})


	t.RegisterConversion("preview", func(img image.Image, opts *Options) (image.Image, error) {
		return t.ResizeImage(img, 600, 400, opts)
	})
}


func (t *ImagingTransformer) DefaultResponsiveConversions() {

	t.RegisterResponsiveImageConversion("responsive", []int{320, 640, 960, 1280, 1600, 1920},
		WithQuality(85),
		WithFit("contain"),
	)
}


func (t *ImagingTransformer) ResizeImage(img image.Image, width, height int, opts *Options) (image.Image, error) {

	if opts.Width > 0 {
		width = opts.Width
	}

	if opts.Height > 0 {
		height = opts.Height
	}

	var result image.Image


	switch opts.Fit {
	case "contain":
		result = imaging.Fit(img, width, height, imaging.Lanczos)
	case "max":
		result = imaging.Resize(img, width, 0, imaging.Lanczos)
	case "fill":
		result = imaging.Fill(img, width, height, imaging.Center, imaging.Lanczos)
	case "stretch":
		result = imaging.Resize(img, width, height, imaging.Lanczos)
	default:

		result = imaging.Fit(img, width, height, imaging.Lanczos)
	}


	if opts.Blur > 0 {
		result = imaging.Blur(result, float64(opts.Blur))
	}

	if opts.Sharpen > 0 {
		result = imaging.Sharpen(result, float64(opts.Sharpen))
	}

	if opts.BrightnessQ != 0 {
		result = imaging.AdjustBrightness(result, float64(opts.BrightnessQ))
	}

	if opts.ContrastQ != 0 {
		result = imaging.AdjustContrast(result, float64(opts.ContrastQ))
	}


	if opts.Border != "" {
		borderColor, err := parseHexColor(opts.Border)
		if err == nil {
			result = imaging.AdjustBrightness(result, 0) 
			result = imaging.Paste(imaging.New(result.Bounds().Dx()+10, result.Bounds().Dy()+10, borderColor), result, image.Pt(5, 5))
		}
	}

	return result, nil
}


func parseHexColor(s string) (c color.NRGBA, err error) {
	c.A = 0xff

	if s[0] != '#' {
		return c, fmt.Errorf("invalid hex color format")
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = fmt.Errorf("invalid hex color format")
		return 0
	}

	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = fmt.Errorf("invalid hex color format")
	}

	return
}
