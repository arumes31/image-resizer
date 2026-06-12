//go:build !cgo

package processor

import (
	"errors"
	"image"
)

func decodeHEIC(path string) (image.Image, error) {
	return nil, errors.New("HEIC/HEIF format requires CGO enabled build with libheif")
}
