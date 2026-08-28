package processor

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
)

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close test image: %v", err)
		}
	}()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
}

func TestIsAllowedImageFile(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"photo.JPG":       true,
		"archive.tar.png": true,
		"script.svg":      false,
		"image.png.exe":   false,
		"no-extension":    false,
	}
	for name, want := range tests {
		if got := IsAllowedImageFile(name); got != want {
			t.Errorf("IsAllowedImageFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestValidateDimensions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		width       int
		height      int
		wantFailure bool
	}{
		{name: "valid", width: 2000, height: 2000},
		{name: "zero", width: 0, height: 10, wantFailure: true},
		{name: "side limit", width: MaxImageDimension + 1, height: 1, wantFailure: true},
		{name: "area limit", width: 7000, height: 7000, wantFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateDimensions(test.width, test.height)
			if (err != nil) != test.wantFailure {
				t.Fatalf("validateDimensions(%d, %d) error = %v", test.width, test.height, err)
			}
		})
	}
}

func TestProcessImageResizesAndEncodes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	output := filepath.Join(dir, "output")
	if err := os.Mkdir(output, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, source, 4, 2)

	result, err := ProcessImage(source, output, &ProcessOptions{
		Operation:  "percentage",
		Percentage: 50,
		Format:     "png",
		Method:     "nearest",
	})
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if result.OriginalSize != "4x2" || result.NewSize != "2x1" {
		t.Fatalf("unexpected dimensions: %#v", result)
	}

	processed, err := imaging.Open(result.NewFilePath)
	if err != nil {
		t.Fatalf("open processed image: %v", err)
	}
	if got := processed.Bounds().Size(); got.X != 2 || got.Y != 1 {
		t.Fatalf("processed dimensions = %v, want 2x1", got)
	}
}

func TestProcessImageRejectsUnsafeInputs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.png")
	if err := os.WriteFile(malformed, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ProcessImage(malformed, dir, &ProcessOptions{}); err == nil {
		t.Fatal("ProcessImage accepted malformed image data")
	}
	if _, err := ProcessImage(malformed, dir, nil); err == nil {
		t.Fatal("ProcessImage accepted nil options")
	}

	source := filepath.Join(dir, "source.png")
	writeTestPNG(t, source, 2, 2)
	_, err := ProcessImage(source, dir, &ProcessOptions{
		Operation: "dimensions",
		Width:     MaxImageDimension + 1,
		Height:    1,
		Format:    "png",
	})
	if err == nil || !strings.Contains(err.Error(), "side limit") {
		t.Fatalf("unsafe output dimensions error = %v", err)
	}
}

func TestProcessImageSanitizesRenameAndFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestPNG(t, source, 2, 2)

	result, err := ProcessImage(source, dir, &ProcessOptions{
		Operation:      "percentage",
		Percentage:     100,
		Format:         ".png",
		RenameTemplate: "../../outside/{name}",
	})
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}
	if strings.Contains(result.ProcessedName, "..") || filepath.Dir(result.NewFilePath) != dir {
		t.Fatalf("unsafe output path: %#v", result)
	}

	_, err = ProcessImage(source, dir, &ProcessOptions{
		Operation:  "percentage",
		Percentage: 100,
		Format:     "../../exe",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unsafe output format error = %v", err)
	}
}

func FuzzIsAllowedImageFile(f *testing.F) {
	for _, seed := range []string{"photo.png", "../photo.JPG", "image.png.exe", "\x00.png"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, filename string) {
		_ = IsAllowedImageFile(filename)
	})
}
