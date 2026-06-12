//go:build !cgo

package processor

import (
	"errors"
	"image"
)

func decodeAVIF(path string) (image.Image, error) {
	return nil, errors.New("AVIF format requires CGO enabled build with libaom")
}

func encodeAVIF(img image.Image, path string, quality int) error {
	return errors.New("AVIF format requires CGO enabled build with libaom")
}
