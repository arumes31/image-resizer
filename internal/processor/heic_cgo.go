//go:build cgo

package processor

import (
	"errors"
	"image"
)

// decodeHEIC decodes a HEIC/HEIF image file.
// NOTE: This stub requires a HEIC decoding library with CGO support.
// When building with CGO enabled and libheif installed, replace the
// function body with the appropriate library call, e.g.:
//
//	import (
//	    "os"
//	    "github.com/go-xmlpath/go-heif/heif"
//	)
//
//	func decodeHEIC(path string) (image.Image, error) {
//	    f, err := os.Open(path)
//	    if err != nil {
//	        return nil, err
//	    }
//	    defer f.Close()
//	    return heif.Decode(f)
//	}
func decodeHEIC(path string) (image.Image, error) {
	return nil, errors.New("HEIC/HEIF format requires libheif — install libheif-dev and rebuild with CGO")
}
