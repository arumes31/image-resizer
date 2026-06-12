package processor

import (
	"errors"
	"image"
)

// decodeJXL decodes a JPEG XL image file.
// There is no mature Go JXL library yet. Use cjxl/djxl tools externally.
func decodeJXL(path string) (image.Image, error) {
	return nil, errors.New("JPEG XL format is not yet supported — use cjxl/djxl tools externally")
}

// encodeJXL encodes an image to JPEG XL format.
// There is no mature Go JXL library yet. Use cjxl/djxl tools externally.
func encodeJXL(img image.Image, path string, quality int) error {
	return errors.New("JPEG XL format is not yet supported — use cjxl/djxl tools externally")
}
