package processor

import (
	"image"

	"github.com/disintegration/imaging"
)

// applyCrop applies smart crop based on the Crop field in ProcessOptions.
// Supports "1:1", "16:9", "4:3" aspect ratios using imaging.Fill with center anchor.
func applyCrop(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Crop == "" || opts.Crop == "none" {
		return img
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	var targetW, targetH int

	switch opts.Crop {
	case "1:1":
		size := w
		if h < w {
			size = h
		}
		targetW, targetH = size, size
	case "16:9":
		if float64(w)/float64(h) > 16.0/9.0 {
			targetH = h
			targetW = int(float64(h) * 16.0 / 9.0)
		} else {
			targetW = w
			targetH = int(float64(w) * 9.0 / 16.0)
		}
	case "4:3":
		if float64(w)/float64(h) > 4.0/3.0 {
			targetH = h
			targetW = int(float64(h) * 4.0 / 3.0)
		} else {
			targetW = w
			targetH = int(float64(w) * 3.0 / 4.0)
		}
	}

	if targetW > 0 && targetH > 0 {
		img = imaging.Fill(img, targetW, targetH, imaging.Center, imaging.Lanczos)
	}

	return img
}

// applyRotation rotates the image by the angle specified in opts.Rotation (0, 90, 180, 270).
func applyRotation(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Rotation != 0 {
		img = imaging.Rotate(img, float64(opts.Rotation), image.Transparent)
	}
	return img
}

// applyFlip flips the image horizontally, vertically, or both based on opts.Flip.
func applyFlip(img image.Image, opts *ProcessOptions) image.Image {
	switch opts.Flip {
	case "h":
		img = imaging.FlipH(img)
	case "v":
		img = imaging.FlipV(img)
	case "both":
		img = imaging.FlipH(img)
		img = imaging.FlipV(img)
	}
	return img
}
