package processor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	p := filepath.Join(dir, "src.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

// writePNGHeader writes a PNG that only declares its dimensions in IHDR,
// the shape of a decompression bomb.
func writePNGHeader(t *testing.T, dir string, w, h uint32) string {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // RGBA
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(ihdr))); err != nil {
		t.Fatal(err)
	}
	chunk := append([]byte("IHDR"), ihdr...)
	buf.Write(chunk)
	if err := binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk)); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "bomb.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProcessImageRenameTemplateTraversal(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")
	if err := os.Mkdir(dest, 0o750); err != nil {
		t.Fatal(err)
	}
	src := writeTestPNG(t, dir, 8, 8)

	for _, tmpl := range []string{"../../../../tmp/pwn", `..\evil`, "a/b", "{name}/.."} {
		opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png", RenameTemplate: tmpl}
		if _, err := ProcessImage(src, "photo.png", dest, opts); !errors.Is(err, ErrInvalidRenameTemplate) {
			t.Errorf("template %q: expected ErrInvalidRenameTemplate, got %v", tmpl, err)
		}
	}

	// A client file name with path components must not escape either.
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png", RenameTemplate: "{name}_x"}
	res, err := ProcessImage(src, "../../etc/pass wd.png", dest, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(res.NewFilePath) != dest {
		t.Errorf("output escaped destination: %s", res.NewFilePath)
	}
	if !strings.HasPrefix(res.ProcessedName, "pass_wd_x_") || !strings.HasSuffix(res.ProcessedName, ".png") {
		t.Errorf("unexpected processed name %q", res.ProcessedName)
	}
}

func TestProcessImageRejectsFormat(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 8, 8)
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "php"}
	if _, err := ProcessImage(src, "a.png", dir, opts); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestProcessImageRejectsHugeInput(t *testing.T) {
	dir := t.TempDir()
	src := writePNGHeader(t, dir, 50000, 50000)
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png"}
	if _, err := ProcessImage(src, "bomb.png", dir, opts); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected ErrImageTooLarge, got %v", err)
	}
}

func TestProcessImageRejectsHugeOutput(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 8, 8)
	opts := &ProcessOptions{Operation: "dimensions", Width: MaxOutputDimension + 1, Height: 10, Format: "png"}
	if _, err := ProcessImage(src, "a.png", dir, opts); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge, got %v", err)
	}
	opts = &ProcessOptions{Operation: "dimensions", Width: MaxOutputDimension, Height: MaxOutputDimension, Format: "png"}
	if _, err := ProcessImage(src, "a.png", dir, opts); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge for pixel budget, got %v", err)
	}
}

func TestProcessImageUniqueNames(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 8, 8)
	opts := &ProcessOptions{Operation: "percentage", Percentage: 50, Format: "png"}
	a, err := ProcessImage(src, "IMG_0001.png", dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ProcessImage(src, "IMG_0001.png", dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.ProcessedName == b.ProcessedName {
		t.Fatalf("expected unique names, got %q twice", a.ProcessedName)
	}
	if a.NewSize != "4x4" {
		t.Errorf("unexpected size %s", a.NewSize)
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":              "photo.jpg",
		"../../etc/passwd":       "passwd",
		`..\..\win.ini`:          "win.ini",
		"my photo (1).png":       "my_photo__1_.png",
		"..":                     "image",
		".hidden":                "hidden",
		"":                       "image",
		strings.Repeat("a", 300): strings.Repeat("a", maxNameLength),
	}
	for in, want := range cases {
		if got := SanitizeFileName(in); got != want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	dir := t.TempDir()
	if _, err := safeJoin(dir, "ok.png"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	for _, bad := range []string{"../x.png", "..", "."} {
		if _, err := safeJoin(dir, bad); err == nil {
			t.Errorf("safeJoin(%q) should fail", bad)
		}
	}
}
