//go:build cgo

package processor

import (
	"errors"
	"image"
)

// decodeAVIF decodes an AVIF image file.
// NOTE: This stub requires the github.com/Kagami/go-avif package which
// needs libaom (CGO). When building with CGO enabled, replace the
// function body with:
//
//	import "github.com/Kagami/go-avif"
//
//	func decodeAVIF(path string) (image.Image, error) {
//	    return avif.DecodeFile(path)
//	}
func decodeAVIF(path string) (image.Image, error) {
	return nil, errors.New("AVIF format requires libaom — install libaom-dev and rebuild with CGO")
}

// encodeAVIF encodes an image to AVIF format.
// NOTE: This stub requires the github.com/Kagami/go-avif package which
// needs libaom (CGO). When building with CGO enabled, replace the
// function body with:
//
//	func encodeAVIF(img image.Image, path string, quality int) error {
//	    return avif.EncodeFile(path, img, &avif.Options{Quality: quality})
//	}
func encodeAVIF(img image.Image, path string, quality int) error {
	return errors.New("AVIF format requires libaom — install libaom-dev and rebuild with CGO")
}
