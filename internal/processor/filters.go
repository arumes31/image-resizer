package processor

import (
	"image"
	"image/color"
	"strings"

	"github.com/disintegration/imaging"
)

// applySepia applies a sepia tone filter to the image.
func applySepia(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			r, g, b := float64(r32>>8), float64(g32>>8), float64(b32>>8)
			newR := clampUint8(int(0.393*r + 0.769*g + 0.189*b))
			newG := clampUint8(int(0.349*r + 0.686*g + 0.168*b))
			newB := clampUint8(int(0.272*r + 0.534*g + 0.131*b))
			dst.Set(x, y, color.RGBA{newR, newG, newB, clampUint8(int(a32 >> 8))})
		}
	}
	return dst
}

// applyFilters applies all artistic filters specified in opts.Filters.
// Supported filters: grayscale, sepia, invert, blur, sharpen, pixelate, noir, vivid.
func applyFilters(img image.Image, opts *ProcessOptions) image.Image {
	for _, filter := range opts.Filters {
		switch strings.ToLower(filter) {
		case "grayscale":
			img = imaging.Grayscale(img)
		case "invert":
			img = imaging.Invert(img)
		case "sepia":
			img = applySepia(img)
		case "blur":
			img = imaging.Blur(img, 2.0)
		case "sharpen":
			img = imaging.Sharpen(img, 1.0)
		case "noir":
			img = imaging.Grayscale(img)
			img = imaging.AdjustContrast(img, 20)
			img = imaging.AdjustBrightness(img, -10)
		case "vivid":
			img = imaging.AdjustSaturation(img, 30)
			img = imaging.AdjustContrast(img, 10)
		case "pixelate":
			if opts.Pixelate > 0 {
				bounds := img.Bounds()
				w, h := bounds.Dx(), bounds.Dy()
				pxSize := opts.Pixelate
				if pxSize < 1 {
					pxSize = 1
				}
				smallW := w / pxSize
				smallH := h / pxSize
				if smallW < 1 {
					smallW = 1
				}
				if smallH < 1 {
					smallH = 1
				}
				img = imaging.Resize(img, smallW, smallH, imaging.NearestNeighbor)
				img = imaging.Resize(img, w, h, imaging.NearestNeighbor)
			}
		}
	}
	return img
}
