package processor

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/jung-kurt/gofpdf"
	"github.com/sergeymakinen/go-ico"
	"golang.org/x/image/font"
)

type ResizeMethod string

const (
	Nearest  ResizeMethod = "nearest"
	Bilinear ResizeMethod = "bilinear"
	Bicubic  ResizeMethod = "bicubic"
	Lanczos  ResizeMethod = "lanczos"
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
			factor := 1.0 - 0.7*math.Pow(dist/maxDist, 2)
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

func ProcessImage(srcPath, destDir string, opts *ProcessOptions) (*ProcessResult, error) {
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

	// BUG-02 FIX: Ensure dimensions are at least 1px to prevent
	// zero-size image generation which causes errors or blank output.
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
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
		wm, wErr := imaging.Open(opts.WatermarkPath)
		if wErr == nil {
			resizedImg = imaging.OverlayCenter(resizedImg, wm, 0.5)
		} else {
			fmt.Printf("Warning: failed to open watermark %s: %v\n", opts.WatermarkPath, wErr)
		}
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
			fontBytes, fErr := os.ReadFile(filepath.Clean(fontPath)) // #nosec G304
			if fErr == nil {
				f, pErr := freetype.ParseFont(fontBytes)
				if pErr == nil {
					// BUG-09 FIX: Use validated color parsing with fallback
					r, g, b := parseHexColor(opts.TextColor)
					fg := image.NewUniform(color.RGBA{r, g, b, 255})

					c := freetype.NewContext()
					c.SetDPI(72)
					c.SetFont(f)
					size := opts.TextSize
					if size == 0 {
						size = 24
					}
					c.SetFontSize(size)
					c.SetClip(dst.Bounds())
					c.SetDst(dst)
					c.SetSrc(fg)
					c.SetHinting(font.HintingFull)

					pt := freetype.Pt(10, dst.Bounds().Dy()-10)
					if _, dErr := c.DrawString(opts.TextOverlay, pt); dErr != nil {
						fmt.Printf("Warning: failed to draw text overlay: %v\n", dErr)
					}
					resizedImg = dst
				} else {
					fmt.Printf("Warning: failed to parse font: %v\n", pErr)
				}
			} else {
				fmt.Printf("Warning: failed to load font at %s: %v\n", fontPath, fErr)
			}
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
			fontBytes, fErr := os.ReadFile(filepath.Clean(fontPath)) // #nosec G304
			if fErr == nil {
				f, pErr := freetype.ParseFont(fontBytes)
				if pErr == nil {
					fg := image.NewUniform(color.RGBA{200, 200, 200, 180})
					c := freetype.NewContext()
					c.SetDPI(72)
					c.SetFont(f)
					c.SetFontSize(12)
					c.SetClip(dst.Bounds())
					c.SetDst(dst)
					c.SetSrc(fg)
					c.SetHinting(font.HintingFull)

					// Position at bottom-right with padding
					textWidth := len(opts.Copyright) * 7 // approximate width
					xPos := dst.Bounds().Dx() - textWidth - 10
					if xPos < 10 {
						xPos = 10
					}
					pt := freetype.Pt(xPos, dst.Bounds().Dy()-10)
					if _, dErr := c.DrawString("© "+opts.Copyright, pt); dErr != nil {
						fmt.Printf("Warning: failed to draw copyright: %v\n", dErr)
					}
					resizedImg = dst
				}
			}
		}
	}

	fileName := filepath.Base(srcPath)
	origNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	ext := strings.ToLower(opts.Format)
	if ext == "" || ext == "pdf" {
		ext = "jpg" // PDF intermediate is JPG
	}

	// BUG-07 FIX: Add timestamp suffix to prevent filename collisions
	// when multiple users upload files with the same name concurrently.
	processedFileName := ""
	if opts.RenameTemplate != "" {
		processedFileName = strings.ReplaceAll(opts.RenameTemplate, "{name}", origNameNoExt)
		processedFileName = fmt.Sprintf("%s_%d.%s", processedFileName, time.Now().UnixMilli(), ext)
	} else {
		processedFileName = fmt.Sprintf("processed_%s_%d.%s", origNameNoExt, time.Now().UnixMilli(), ext)
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
		defer out.Close()
		var webpOpts *webp.Options
		if opts.Quality > 0 {
			webpOpts = &webp.Options{Lossless: false, Quality: float32(opts.Quality)}
		}
		eErr := webp.Encode(out, resizedImg, webpOpts)
		if eErr != nil {
			return nil, fmt.Errorf("failed to save webp image: %w", eErr)
		}
	case "ico":
		out, cErr := os.Create(filepath.Clean(destPath)) // #nosec G304
		if cErr != nil {
			return nil, fmt.Errorf("failed to create ico file: %w", cErr)
		}
		defer out.Close()
		eErr := ico.Encode(out, resizedImg)
		if eErr != nil {
			return nil, fmt.Errorf("failed to save ico image: %w", eErr)
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
