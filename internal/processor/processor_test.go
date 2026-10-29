package processor

import (
	"bytes"
	"context"
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

	"github.com/disintegration/imaging"
	"github.com/sergeymakinen/go-ico"
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
		if _, err := ProcessImage(context.Background(), src, "photo.png", dest, opts); !errors.Is(err, ErrInvalidRenameTemplate) {
			t.Errorf("template %q: expected ErrInvalidRenameTemplate, got %v", tmpl, err)
		}
	}

	// A client file name with path components must not escape either.
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png", RenameTemplate: "{name}_x"}
	res, err := ProcessImage(context.Background(), src, "../../etc/pass wd.png", dest, opts)
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
	if _, err := ProcessImage(context.Background(), src, "a.png", dir, opts); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestProcessImageRejectsHugeInput(t *testing.T) {
	dir := t.TempDir()
	src := writePNGHeader(t, dir, 50000, 50000)
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png"}
	if _, err := ProcessImage(context.Background(), src, "bomb.png", dir, opts); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected ErrImageTooLarge, got %v", err)
	}
}

func TestProcessImageRejectsHugeOutput(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 8, 8)
	opts := &ProcessOptions{Operation: "dimensions", Width: MaxOutputDimension + 1, Height: 10, Format: "png"}
	if _, err := ProcessImage(context.Background(), src, "a.png", dir, opts); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge, got %v", err)
	}
	opts = &ProcessOptions{Operation: "dimensions", Width: MaxOutputDimension, Height: MaxOutputDimension, Format: "png"}
	if _, err := ProcessImage(context.Background(), src, "a.png", dir, opts); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge for pixel budget, got %v", err)
	}
}

func TestProcessImageUniqueNames(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 8, 8)
	opts := &ProcessOptions{Operation: "percentage", Percentage: 50, Format: "png"}
	a, err := ProcessImage(context.Background(), src, "IMG_0001.png", dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ProcessImage(context.Background(), src, "IMG_0001.png", dir, opts)
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
		"a..b.png":               "a.b.png",
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

func TestNormalizeFilters(t *testing.T) {
	got := normalizeFilters([]string{"blur", "BLUR", " blur ", "nope", "sepia", "blur"})
	if strings.Join(got, ",") != "blur,sepia" {
		t.Fatalf("got %v", got)
	}
}

func TestProcessImageHonorsCanceledContext(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 4, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := &ProcessOptions{Operation: "percentage", Percentage: 100, Format: "png"}
	if _, err := ProcessImage(ctx, src, "a.png", dir, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// writeICO writes an ICO directory with one entry declaring size and offset.
func writeICO(t *testing.T, dir string, size, offset uint32, payload []byte) string {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("\x00\x00\x01\x00")
	if err := binary.Write(&buf, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatal(err)
	}
	entry := make([]byte, 16)
	binary.LittleEndian.PutUint32(entry[8:], size)
	binary.LittleEndian.PutUint32(entry[12:], offset)
	buf.Write(entry)
	buf.Write(payload)
	p := filepath.Join(dir, "x.ico")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckImageDimensionsRejectsOversizedICOEntry(t *testing.T) {
	dir := t.TempDir()
	// A 22-byte file declaring a 4 GiB entry must be rejected before decoding.
	p := writeICO(t, dir, 0xFFFFFFFF, 22, nil)
	if err := CheckImageDimensions(p); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected ErrInvalidImage, got %v", err)
	}
	// An entry pointing back into the directory is rejected too.
	p = writeICO(t, dir, 4, 2, []byte{0, 0, 0, 0})
	if err := CheckImageDimensions(p); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected ErrInvalidImage, got %v", err)
	}
}

func TestCheckImageDimensionsAcceptsValidICO(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := ico.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "ok.ico")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckImageDimensions(p); err != nil {
		t.Fatalf("valid ICO rejected: %v", err)
	}
}

func TestResizeBoundedPicksSmallIntermediate(t *testing.T) {
	// Horizontal-first would allocate 4000x4000; vertical-first only 1x10.
	img := image.NewNRGBA(image.Rect(0, 0, 1, 4000))
	out, err := resizeBounded(img, 4000, 10, imaging.Lanczos)
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 4000 || b.Dy() != 10 {
		t.Fatalf("got %v", b)
	}
}

func TestResizeBoundedRejectsHugeIntermediate(t *testing.T) {
	// Both pass orders would exceed MaxIntermediatePixels. The check runs on
	// bounds alone, so no pixels need to be allocated.
	img := boundsOnly{image.Rect(0, 0, 5000, 9000)}
	if _, err := resizeBounded(img, 8999, 8999, imaging.Lanczos); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected ErrOutputTooLarge, got %v", err)
	}
}

type boundsOnly struct{ r image.Rectangle }

func (b boundsOnly) ColorModel() color.Model { return color.NRGBAModel }
func (b boundsOnly) Bounds() image.Rectangle { return b.r }
func (b boundsOnly) At(int, int) color.Color { return color.Transparent }

func TestFillBoundedCropsFirst(t *testing.T) {
	// imaging.Fill would scale this 1x40000 strip to 1000x40000000 first.
	img := image.NewNRGBA(image.Rect(0, 0, 1, 40000))
	out, err := fillBounded(img, 1000, 1000, imaging.Lanczos)
	if err != nil {
		t.Fatal(err)
	}
	if b := out.Bounds(); b.Dx() != 1000 || b.Dy() != 1000 {
		t.Fatalf("got %v", b)
	}
}
