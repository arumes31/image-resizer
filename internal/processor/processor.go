package processor

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/jung-kurt/gofpdf"
	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type ResizeMethod string

const (
	Nearest  ResizeMethod = "nearest"
	Bilinear ResizeMethod = "bilinear"
	Bicubic  ResizeMethod = "bicubic"
	Lanczos  ResizeMethod = "lanczos"

	// MaxImageDimension and MaxImagePixels bound decoded and generated images.
	// These limits prevent small compressed uploads from causing excessive
	// allocations during decoding or resizing.
	MaxImageDimension = 12_000
	MaxImagePixels    = int64(40_000_000)
	maxFontBytes      = int64(16 << 20)
)

var ResizeMethods = map[string]imaging.ResampleFilter{
	"nearest":  imaging.NearestNeighbor,
	"bilinear": imaging.Linear,
	"bicubic":  imaging.CatmullRom,
	"lanczos":  imaging.Lanczos,
}

// AllowedExtensions defines which file types may be uploaded.
var AllowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
	".ico":  true,
}

type ProcessResult struct {
	OriginalName  string `json:"originalName"`
	ProcessedName string `json:"processedName"`
	OriginalSize  string `json:"originalSize"`
	NewSize       string `json:"newSize"`
	NewFilePath   string `json:"newFilePath"`
	Error         string `json:"error,omitempty"`
}

type ProcessOptions struct {
	Operation      string
	Percentage     int
	Width          int
	Height         int
	Quality        int
	Method         string
	Format         string
	Rotation       int      // 0, 90, 180, 270
	Flip           string   // "h", "v", "both", ""
	Filters        []string // "grayscale", "sepia", "invert", "blur", "sharpen", "pixelate", "noir", "vivid"
	WatermarkPath  string
	WatermarkPos   string // "center", "top-left", etc.
	TextOverlay    string
	TextColor      string // hex
	TextSize       float64
	StripEXIF      bool
	Copyright      string
	Brightness     float64 // -100 to 100
	Contrast       float64 // -100 to 100
	Saturation     float64 // -100 to 100
	Pixelate       int     // 0 (off) to 100
	Crop           string  // "1:1", "16:9", "4:3", "none"
	Vignette       bool
	RenameTemplate string
}

// IsAllowedImageFile checks whether the file extension is an allowed image type.
func IsAllowedImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return AllowedExtensions[ext]
}

func validateDimensions(width, height int) error {
	if width < 1 || height < 1 {
		return fmt.Errorf("image dimensions must be positive")
	}
	if width > MaxImageDimension || height > MaxImageDimension {
		return fmt.Errorf("image dimensions %dx%d exceed the %d pixel side limit", width, height, MaxImageDimension)
	}
	if int64(width)*int64(height) > MaxImagePixels {
		return fmt.Errorf("image dimensions %dx%d exceed the %d pixel area limit", width, height, MaxImagePixels)
	}
	return nil
}

func validateImageFile(path string) error {
	file, err := os.Open(filepath.Clean(path)) // #nosec G304 -- caller supplies an application-owned path.
	if err != nil {
		return fmt.Errorf("open image header: %w", err)
	}
	imageConfig, _, err := image.DecodeConfig(file)
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("decode image header: %w", errors.Join(err, closeErr))
	}
	if closeErr != nil {
		return fmt.Errorf("close image header: %w", closeErr)
	}
	return validateDimensions(imageConfig.Width, imageConfig.Height)
}

func getPlatformFontPath() string {
	switch runtime.GOOS {
	case "windows":
		return "C:\\Windows\\Fonts\\arial.ttf"
	case "darwin":
		return "/Library/Fonts/Arial.ttf"
	default: // Linux and others
		paths := []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
			"/usr/share/fonts/TTF/DejaVuSans.ttf",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func CreatePDF(imagePaths []string, destPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	for _, path := range imagePaths {
		pdf.AddPage()
		// Auto-size image to fit A4 width
		pdf.ImageOptions(path, 10, 10, 190, 0, false, gofpdf.ImageOptions{ImageType: "", ReadDpi: true}, 0, "")
	}
	return pdf.OutputFileAndClose(destPath)
}

// clampUint8 clamps an integer value to the [0, 255] range and returns a uint8.
// BUG-01 FIX: Replaces the old min() function which had an incorrect type
// signature (returning uint8 from int params) and shadowed the Go 1.21+ builtin.
func clampUint8(val int) uint8 {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return uint8(val)
}

func applySepia(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r32, g32, b32, a32 := img.At(x, y).RGBA()
			r, g, b := float64(r32>>8), float64(g32>>8), float64(b32>>8)
			newR := clampUint8(int(0.393*r + 0.769*g + 0.189*b))
			newG := clampUint8(int(0.349*r + 0.686*g + 0.168*b))
			newB := clampUint8(int(0.272*r + 0.534*g + 0.131*b))
			dst.Set(x, y, color.RGBA{newR, newG, newB, clampUint8(int(a32 >> 8))})
		}
	}
	return dst
}

// applyVignette adds a radial darkening effect around the edges of the image.
// BUG-10 FIX: Vignette was defined in ProcessOptions but never implemented.
func applyVignette(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	cx := float64(bounds.Dx()) / 2.0
	cy := float64(bounds.Dy()) / 2.0
	maxDist := math.Sqrt(cx*cx + cy*cy)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			// Vignette strength: 0 at center, up to ~0.7 at corners
			factor := 1.0 - 0.7*(dist/maxDist)*(dist/maxDist)
			if factor < 0 {
				factor = 0
			}

			cR, cG, cB, cA := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: uint8(float64(cR>>8) * factor),
				G: uint8(float64(cG>>8) * factor),
				B: uint8(float64(cB>>8) * factor),
				A: clampUint8(int(cA >> 8)),
			})
		}
	}
	return dst
}

// hexColorRegex validates #RRGGBB hex color strings.
var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var unsafeFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// parseHexColor parses a #RRGGBB hex color string into r, g, b uint8 values.
// BUG-09 FIX: Previously used fmt.Sscanf which silently ignored parse errors,
// potentially producing invisible text (e.g. black on dark background).
// Now validates format and falls back to white on invalid input.
func parseHexColor(hexStr string) (r, g, b uint8) {
	r, g, b = 255, 255, 255 // default white
	if hexStr == "" {
		return
	}
	if !hexColorRegex.MatchString(hexStr) {
		fmt.Printf("Warning: invalid text color format %q, falling back to white\n", hexStr)
		return
	}
	_, err := fmt.Sscanf(hexStr, "#%02x%02x%02x", &r, &g, &b)
	if err != nil {
		fmt.Printf("Warning: failed to parse text color %q: %v, falling back to white\n", hexStr, err)
		r, g, b = 255, 255, 255
	}
	return
}

func normalizeOutputFormat(requestedFormat, sourcePath string) (string, error) {
	extension := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(requestedFormat)), ".")
	if extension == "" {
		extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(sourcePath)), ".")
	}
	if extension == "pdf" {
		return "jpg", nil
	}
	if !AllowedExtensions["."+extension] {
		return "", fmt.Errorf("unsupported output format %q", requestedFormat)
	}
	return extension, nil
}

func sanitizeOutputBase(template, originalName string) string {
	candidate := strings.ReplaceAll(template, "{name}", originalName)
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	candidate = filepath.Base(candidate)
	candidate = strings.TrimSuffix(candidate, filepath.Ext(candidate))
	candidate = unsafeFilenameRegex.ReplaceAllString(candidate, "_")
	candidate = strings.Trim(candidate, "._-")
	if candidate == "" {
		candidate = unsafeFilenameRegex.ReplaceAllString(originalName, "_")
	}
	if len(candidate) > 100 {
		candidate = candidate[:100]
	}
	return candidate
}

func drawTrueTypeText(
	dst draw.Image,
	fontPath string,
	text string,
	fontSize float64,
	foreground image.Image,
	x int,
	y int,
) error {
	fontBytes, err := readBoundedFile(fontPath, maxFontBytes)
	if err != nil {
		return fmt.Errorf("load font: %w", err)
	}

	parsedFont, err := opentype.Parse(fontBytes)
	if err != nil {
		return fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return fmt.Errorf("create font face: %w", err)
	}
	drawer := font.Drawer{
		Dst:  dst,
		Src:  foreground,
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
	if err := face.Close(); err != nil {
		return fmt.Errorf("close font face: %w", err)
	}
	return nil
}

// readBoundedFile opens only the named file within its parent directory. Using
// os.Root prevents a replaced symlink from escaping that directory between path
// validation and open, while the size limit bounds memory used by font parsing.
func readBoundedFile(path string, maxBytes int64) ([]byte, error) {
	if maxBytes < 1 {
		return nil, fmt.Errorf("maximum file size must be positive")
	}
	cleanPath := filepath.Clean(path)
	root, err := os.OpenRoot(filepath.Dir(cleanPath))
	if err != nil {
		return nil, fmt.Errorf("open parent directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	file, err := root.Open(filepath.Base(cleanPath))
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	contents, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if int64(len(contents)) > maxBytes {
		return nil, fmt.Errorf("file exceeds %d-byte limit", maxBytes)
	}
	return contents, nil
}

func ProcessImage(srcPath, destDir string, opts *ProcessOptions) (*ProcessResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("process options are required")
	}
	if err := validateImageFile(srcPath); err != nil {
		return nil, fmt.Errorf("validate source image: %w", err)
	}

	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}

	origWidthBeforeTransforms := img.Bounds().Dx()
	origHeightBeforeTransforms := img.Bounds().Dy()

	// BUG-12 FIX: StripEXIF is now functional.
	// imaging.Open with AutoOrientation already handles EXIF orientation,
	// and re-encoding the image through the pipeline naturally strips EXIF
	// metadata since we only preserve pixel data. For explicit stripping,
	// we convert to RGBA and back which ensures no metadata survives.
	if opts.StripEXIF {
		// Force re-encode through RGBA to guarantee all EXIF is removed
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
		img = rgba
	}

	// 1. Composition: Smart Crop
	if opts.Crop != "" && opts.Crop != "none" {
		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		var targetW, targetH int
		switch opts.Crop {
		case "1:1":
			size := w
			if h < w {
				size = h
			}
			targetW, targetH = size, size
		case "16:9":
			if float64(w)/float64(h) > 16.0/9.0 {
				targetH = h
				targetW = int(float64(h) * 16.0 / 9.0)
			} else {
				targetW = w
				targetH = int(float64(w) * 9.0 / 16.0)
			}
		case "4:3":
			if float64(w)/float64(h) > 4.0/3.0 {
				targetH = h
				targetW = int(float64(h) * 4.0 / 3.0)
			} else {
				targetW = w
				targetH = int(float64(w) * 3.0 / 4.0)
			}
		}
		if targetW > 0 && targetH > 0 {
			img = imaging.Fill(img, targetW, targetH, imaging.Center, imaging.Lanczos)
		}
	}

	// 2. Transformations: Rotation
	if opts.Rotation != 0 {
		img = imaging.Rotate(img, float64(opts.Rotation), image.Transparent)
	}

	// 3. Transformations: Flipping
	switch opts.Flip {
	case "h":
		img = imaging.FlipH(img)
	case "v":
		img = imaging.FlipV(img)
	case "both":
		img = imaging.FlipH(img)
		img = imaging.FlipV(img)
	}

	// 4. Filters
	for _, filter := range opts.Filters {
		switch strings.ToLower(filter) {
		case "grayscale":
			img = imaging.Grayscale(img)
		case "invert":
			img = imaging.Invert(img)
		case "sepia":
			img = applySepia(img)
		case "blur":
			img = imaging.Blur(img, 2.0)
		case "sharpen":
			img = imaging.Sharpen(img, 1.0)
		case "noir":
			img = imaging.Grayscale(img)
			img = imaging.AdjustContrast(img, 20)
			img = imaging.AdjustBrightness(img, -10)
		case "vivid":
			img = imaging.AdjustSaturation(img, 30)
			img = imaging.AdjustContrast(img, 10)
		case "pixelate":
			if opts.Pixelate > 0 {
				bounds := img.Bounds()
				w, h := bounds.Dx(), bounds.Dy()
				pxSize := opts.Pixelate
				if pxSize < 1 {
					pxSize = 1
				}
				smallW := w / pxSize
				smallH := h / pxSize
				if smallW < 1 {
					smallW = 1
				}
				if smallH < 1 {
					smallH = 1
				}
				img = imaging.Resize(img, smallW, smallH, imaging.NearestNeighbor)
				img = imaging.Resize(img, w, h, imaging.NearestNeighbor)
			}
		}
	}

	// 5. Dynamic Adjustments
	if opts.Brightness != 0 {
		img = imaging.AdjustBrightness(img, opts.Brightness)
	}
	if opts.Contrast != 0 {
		img = imaging.AdjustContrast(img, opts.Contrast)
	}
	if opts.Saturation != 0 {
		img = imaging.AdjustSaturation(img, opts.Saturation)
	}

	// 6. Vignette effect
	// BUG-10 FIX: Vignette was defined but never applied.
	if opts.Vignette {
		img = applyVignette(img)
	}

	origBounds := img.Bounds()
	origWidth, origHeight := origBounds.Dx(), origBounds.Dy()

	var newWidth, newHeight int
	// BUG-02 FIX: Guard against zero dimensions which would produce
	// a 0×0 image or panic in imaging.Resize. Also handle the "fill"
	// operation mode used by social presets (BUG-13 FIX).
	switch opts.Operation {
	case "percentage":
		newWidth = int(float64(origWidth) * float64(opts.Percentage) / 100)
		newHeight = int(float64(origHeight) * float64(opts.Percentage) / 100)
	case "dimensions":
		newWidth = opts.Width
		newHeight = opts.Height
	case "fill":
		// BUG-13 FIX: Social presets use "fill" mode which crops to fit
		// the exact dimensions rather than stretching the image.
		newWidth = opts.Width
		newHeight = opts.Height
	default:
		newWidth = origWidth
		newHeight = origHeight
	}

	if dimensionErr := validateDimensions(newWidth, newHeight); dimensionErr != nil {
		return nil, fmt.Errorf("invalid output dimensions: %w", dimensionErr)
	}

	resampleFilter := imaging.Lanczos
	if f, ok := ResizeMethods[strings.ToLower(opts.Method)]; ok {
		resampleFilter = f
	}

	var resizedImg image.Image
	// BUG-13 FIX: Use imaging.Fill for "fill" operation (social presets)
	// which crops to exact dimensions, vs imaging.Resize which stretches.
	if opts.Operation == "fill" && newWidth > 0 && newHeight > 0 {
		resizedImg = imaging.Fill(img, newWidth, newHeight, imaging.Center, resampleFilter)
	} else {
		resizedImg = imaging.Resize(img, newWidth, newHeight, resampleFilter)
	}

	// 7. Watermarking
	if opts.WatermarkPath != "" {
		if validationErr := validateImageFile(opts.WatermarkPath); validationErr != nil {
			return nil, fmt.Errorf("validate watermark: %w", validationErr)
		}
		wm, watermarkErr := imaging.Open(opts.WatermarkPath)
		if watermarkErr != nil {
			return nil, fmt.Errorf("open watermark: %w", watermarkErr)
		}
		resizedImg = imaging.OverlayCenter(resizedImg, wm, 0.5)
	}

	// 8. Text Overlay
	if opts.TextOverlay != "" {
		dst := image.NewRGBA(resizedImg.Bounds())
		draw.Draw(dst, dst.Bounds(), resizedImg, image.Point{}, draw.Src)

		fontPath := os.Getenv("FONT_PATH")
		if fontPath == "" {
			fontPath = getPlatformFontPath()
		}

		if fontPath != "" {
			// BUG-09 FIX: Use validated color parsing with fallback.
			r, g, b := parseHexColor(opts.TextColor)
			fontSize := opts.TextSize
			if fontSize == 0 {
				fontSize = 24
			}
			if drawErr := drawTrueTypeText(
				dst,
				fontPath,
				opts.TextOverlay,
				fontSize,
				image.NewUniform(color.RGBA{r, g, b, 255}),
				10,
				dst.Bounds().Dy()-10,
			); drawErr != nil {
				fmt.Printf("Warning: failed to draw text overlay: %v\n", drawErr)
			}
			resizedImg = dst
		} else {
			fmt.Println("Warning: No suitable font found for text overlay")
		}
	}

	// 9. Copyright text overlay (if provided, add as small text at bottom-right)
	// BUG-11 FIX: Copyright field was defined but never used.
	if opts.Copyright != "" {
		dst := image.NewRGBA(resizedImg.Bounds())
		draw.Draw(dst, dst.Bounds(), resizedImg, image.Point{}, draw.Src)

		fontPath := os.Getenv("FONT_PATH")
		if fontPath == "" {
			fontPath = getPlatformFontPath()
		}

		if fontPath != "" {
			// Position at bottom-right with padding.
			textWidth := len(opts.Copyright) * 7 // approximate width
			xPos := dst.Bounds().Dx() - textWidth - 10
			if xPos < 10 {
				xPos = 10
			}
			if drawErr := drawTrueTypeText(
				dst,
				fontPath,
				"© "+opts.Copyright,
				12,
				image.NewUniform(color.RGBA{200, 200, 200, 180}),
				xPos,
				dst.Bounds().Dy()-10,
			); drawErr != nil {
				fmt.Printf("Warning: failed to draw copyright: %v\n", drawErr)
			}
			resizedImg = dst
		}
	}

	fileName := filepath.Base(srcPath)
	origNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	ext, err := normalizeOutputFormat(opts.Format, srcPath)
	if err != nil {
		return nil, err
	}

	// BUG-07 FIX: Add timestamp suffix to prevent filename collisions
	// when multiple users upload files with the same name concurrently.
	processedFileName := ""
	if opts.RenameTemplate != "" {
		baseName := sanitizeOutputBase(opts.RenameTemplate, origNameNoExt)
		processedFileName = fmt.Sprintf("%s_%d.%s", baseName, time.Now().UnixNano(), ext)
	} else {
		baseName := sanitizeOutputBase(origNameNoExt, "image")
		processedFileName = fmt.Sprintf("processed_%s_%d.%s", baseName, time.Now().UnixNano(), ext)
	}

	destPath := filepath.Join(destDir, processedFileName)

	// BUG-03 FIX: Only apply JPEG quality option when the output format
	// is JPEG. Previously, JPEGQuality was added for all formats which
	// was misleading and could cause issues.
	var saveOpts []imaging.EncodeOption
	if (ext == "jpg" || ext == "jpeg") && opts.Quality > 0 {
		saveOpts = append(saveOpts, imaging.JPEGQuality(opts.Quality))
	}

	switch ext {
	case "webp":
		out, cErr := os.Create(filepath.Clean(destPath)) // #nosec G304
		if cErr != nil {
			return nil, fmt.Errorf("failed to create webp file: %w", cErr)
		}
		var webpOpts *webp.Options
		if opts.Quality > 0 {
			webpOpts = &webp.Options{Lossless: false, Quality: float32(opts.Quality)}
		}
		eErr := webp.Encode(out, resizedImg, webpOpts)
		closeErr := out.Close()
		if eErr != nil {
			return nil, fmt.Errorf("failed to save webp image: %w", errors.Join(eErr, closeErr))
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close webp image: %w", closeErr)
		}
	case "ico":
		out, cErr := os.Create(filepath.Clean(destPath)) // #nosec G304
		if cErr != nil {
			return nil, fmt.Errorf("failed to create ico file: %w", cErr)
		}
		eErr := ico.Encode(out, resizedImg)
		closeErr := out.Close()
		if eErr != nil {
			return nil, fmt.Errorf("failed to save ico image: %w", errors.Join(eErr, closeErr))
		}
		if closeErr != nil {
			return nil, fmt.Errorf("failed to close ico image: %w", closeErr)
		}
	default:
		sErr := imaging.Save(resizedImg, destPath, saveOpts...)
		if sErr != nil {
			return nil, fmt.Errorf("failed to save image: %w", sErr)
		}
	}

	return &ProcessResult{
		OriginalName:  fileName,
		ProcessedName: processedFileName,
		OriginalSize:  fmt.Sprintf("%dx%d", origWidthBeforeTransforms, origHeightBeforeTransforms),
		NewSize:       fmt.Sprintf("%dx%d", newWidth, newHeight),
		NewFilePath:   destPath,
	}, nil
}
