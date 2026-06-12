//go:build !cgo

package processor

import (
	"errors"
	"image"
)

func decodeRAW(path string) (image.Image, error) {
	return nil, errors.New("RAW format requires CGO enabled build with libraw")
}
