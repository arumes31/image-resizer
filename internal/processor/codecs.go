package processor

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/tiff"
)

// formatSupportsAlpha returns true for formats that support transparency.
func formatSupportsAlpha(format string) bool {
	switch strings.ToLower(format) {
	case "png", "webp", "gif", "tiff", "tif", "avif":
		return true
	default:
		return false
	}
}

// compositeOnWhite creates a white NRGBA image and draws the source onto it.
// This is used when saving images with transparency to formats that don't support alpha.
func compositeOnWhite(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	white := image.NewUniform(color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	result := image.NewNRGBA(bounds)
	// Fill with white
	draw.Draw(result, bounds, white, image.Point{}, draw.Src)
	// Composite the image on top
	draw.Draw(result, bounds, img, bounds.Min, draw.Over)
	return result
}

// getOutputFormat determines the output file format/extension.
// Falls back to "jpg" if no format is specified or if PDF is requested (PDF uses JPG intermediates).
func getOutputFormat(filename string, opts *ProcessOptions) string {
	ext := strings.ToLower(opts.Format)
	if ext == "" || ext == "pdf" {
		ext = "jpg" // PDF intermediate is JPG
	}
	return ext
}

// saveImage saves the image to the specified path using the format determined by opts.
func saveImage(img image.Image, outputPath string, opts *ProcessOptions) error {
	ext := getOutputFormat("", opts)

	// If image has transparency and output format doesn't support alpha,
	// composite onto a white background so transparent pixels appear as white
	// instead of black (which is the default for non-alpha encoders like JPEG).
	if !formatSupportsAlpha(ext) {
		img = compositeOnWhite(img)
	}

	// BUG-03 FIX: Only apply JPEG quality option when the output format
	// is JPEG. Previously, JPEGQuality was added for all formats which
	// was misleading and could cause issues.
	var saveOpts []imaging.EncodeOption
	if (ext == "jpg" || ext == "jpeg") && opts.Quality > 0 {
		saveOpts = append(saveOpts, imaging.JPEGQuality(opts.Quality))
	}

	switch ext {
	case "webp":
		out, cErr := os.Create(filepath.Clean(outputPath)) // #nosec G304
		if cErr != nil {
			return fmt.Errorf("failed to create webp file: %w", cErr)
		}
		defer out.Close()
		var webpOpts *webp.Options
		if opts.LosslessWebP {
			// Item 39: Lossless WebP encoding
			webpOpts = &webp.Options{Lossless: true, Quality: 100}
		} else if opts.Quality > 0 {
			webpOpts = &webp.Options{Lossless: false, Quality: float32(opts.Quality)}
		}
		eErr := webp.Encode(out, img, webpOpts)
		if eErr != nil {
			return fmt.Errorf("failed to save webp image: %w", eErr)
		}
	case "ico":
		// Item 30: Use multi-size ICO bundler when ICOSizes is specified
		if opts.ICOSizes != "" {
			return encodeICOMultiSize(img, outputPath, opts)
		}
		out, cErr := os.Create(filepath.Clean(outputPath)) // #nosec G304
		if cErr != nil {
			return fmt.Errorf("failed to create ico file: %w", cErr)
		}
		defer out.Close()
		eErr := ico.Encode(out, img)
		if eErr != nil {
			return fmt.Errorf("failed to save ico image: %w", eErr)
		}
	case "avif":
		return encodeAVIF(img, outputPath, opts.Quality)
	case "heic":
		return fmt.Errorf("HEIC output is not supported — use as input format only")
	case "jxl":
		return encodeJXL(img, outputPath, opts.Quality)
	case "svg":
		return fmt.Errorf("SVG output (vectorization) is not yet supported — use potrace or similar tools externally")
	case "tiff":
		out, cErr := os.Create(filepath.Clean(outputPath)) // #nosec G304
		if cErr != nil {
			return fmt.Errorf("failed to create tiff file: %w", cErr)
		}
		defer out.Close()
		eErr := tiff.Encode(out, img, &tiff.Options{Compression: tiff.Deflate})
		if eErr != nil {
			return fmt.Errorf("failed to save tiff image: %w", eErr)
		}
	default:
		sErr := imaging.Save(img, outputPath, saveOpts...)
		if sErr != nil {
			return fmt.Errorf("failed to save image: %w", sErr)
		}
	}

	return nil
}
