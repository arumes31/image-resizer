//go:build cgo

package processor

import (
	"errors"
	"image"
)

// decodeRAW decodes a RAW camera image file (CR2, CR3, NEF, ARW, DNG).
// NOTE: This stub requires libraw (CGO). When building with CGO enabled
// and libraw installed, replace the function body with the appropriate
// libraw Go binding call.
func decodeRAW(path string) (image.Image, error) {
	return nil, errors.New("RAW format support requires libraw — convert to TIFF/JPEG first")
}
