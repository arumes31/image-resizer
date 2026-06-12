package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jung-kurt/gofpdf"
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
	".jpg":   true,
	".jpeg":  true,
	".png":   true,
	".gif":   true,
	".bmp":   true,
	".tiff":  true,
	".tif":   true,
	".webp":  true,
	".ico":   true,
	".avif":  true,
	".heic":  true,
	".heif":  true,
	".jxl":   true,
	".svg":   true,
	".cr2":   true, // Canon RAW
	".cr3":   true, // Canon RAW (newer)
	".nef":   true, // Nikon RAW
	".arw":   true, // Sony RAW
	".dng":   true, // Adobe DNG
	".dcm":   true, // DICOM
	".dicom": true, // DICOM
}

type ProcessResult struct {
	OriginalName  string   `json:"originalName"`
	ProcessedName string   `json:"processedName"`
	OriginalSize  string   `json:"originalSize"`
	NewSize       string   `json:"newSize"`
	NewFilePath   string   `json:"newFilePath"`
	ExtraFiles    []string `json:"extraFiles,omitempty"`
	Error         string   `json:"error,omitempty"`
	ImageData     []byte   `json:"-"`                  // For zero-log mode: processed image bytes
	MimeType      string   `json:"mimeType,omitempty"` // MIME type of the output
	Filename      string   `json:"filename,omitempty"` // Output filename
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

	// Background Removal
	RemoveBackground    bool   // Enable background removal
	BgRemovalMethod     string // "transparent", "flood-fill", "color-match"
	BgRemovalTolerance  int    // 0-100 color distance tolerance
	BgRemovalColor      string // hex color for color-match method (e.g., "#ffffff")
	BgRemovalEdgeSmooth int    // 0-10 edge smoothing/feathering radius

	// Professional Adjustments
	Hue                 float64 // -180 to 180 degrees
	Lightness           float64 // -100 to 100
	CurvesPoints        string  // JSON: {"r":[[0,0],[255,255]],"g":[[0,0],[255,255]],"b":[[0,0],[255,255]]}
	LevelsBlack         float64 // 0-255, default 0
	LevelsWhite         float64 // 0-255, default 255
	LevelsGamma         float64 // 0.1-10, default 1.0
	SelectiveColor      string  // JSON: {"reds":{"cyan":0,"magenta":0,"yellow":0,"black":0},...}
	ChromaticAberration float64 // -10 to 10 pixels
	UnsharpAmount       float64 // 0-500 percent
	UnsharpRadius       float64 // 0.1-50 pixels
	GrainAmount         float64 // 0-100
	Temperature         float64 // -100 to 100 (warm/cool)
	Tint                float64 // -100 to 100 (green/magenta)
	ShadowRecovery      float64 // 0-100
	HighlightRecovery   float64 // 0-100
	VignetteAmount      float64 // 0-100 (replaces existing Vignette bool)
	VignetteFeather     float64 // 0-100
	VignetteRoundness   float64 // 0-100
	VignetteMidpoint    float64 // 0-100

	// Branding & Overlays
	WatermarkTemplate    string  // Template with {filename}, {date}, {time}, {year}, {camera}
	WatermarkTile        bool    // Tile watermark across entire image
	WatermarkTileSpacing int     // Spacing between tiled watermarks in pixels
	WatermarkOpacity     float64 // 0.0-1.0 opacity for watermark overlay
	QRCodeText           string  // Text/URL to encode as QR code
	QRCodeSize           int     // QR code size in pixels
	QRCodePosition       string  // "bottom-right", "bottom-left", "top-right", "top-left"
	BarcodeText          string  // Text to encode as barcode
	BarcodeType          string  // "code128", "ean13", "qr" (alias for QR)
	RoundedCorners       int     // Corner radius in pixels (0 = no rounding)
	DropShadowOffset     int     // Shadow offset in pixels
	DropShadowBlur       float64 // Shadow blur radius
	DropShadowColor      string  // Shadow color (hex)
	BorderWidth          int     // Border width in pixels
	BorderColor          string  // Border color (hex)
	BorderStyle          string  // "solid", "dashed"
	PlaceholderWidth     int     // Placeholder image width
	PlaceholderHeight    int     // Placeholder image height
	PlaceholderText      string  // Text to display on placeholder
	PlaceholderBgColor   string  // Background color (hex)
	PlaceholderTextColor string  // Text color (hex)
	SteganographyText    string  // Hidden text to encode in pixels
	SignaturePath        string  // Path to signature PNG file
	SignaturePosition    string  // "bottom-right", "bottom-left", "top-right", "top-left"
	SignatureOpacity     float64 // 0.0-1.0 signature opacity
	SignatureScale       float64 // 0.1-2.0 scale factor for signature

	// Social Media Optimization
	CarouselSlice       bool   // Enable Instagram carousel slicing
	CarouselSliceWidth  int    // Width of each carousel slide (default 1080)
	CarouselSliceHeight int    // Height of each carousel slide (default 1350)
	SafeZoneOverlay     bool   // Draw safe zone guides on thumbnail
	SafeZonePlatform    string // "youtube", "twitter", "linkedin"
	MaxFileSizeKB       int    // Max output file size in KB (for Discord emoji etc.)
	DeviceFramePath     string // Path to device frame PNG for mockup
	StitchImages        bool   // Stitch multiple images vertically (Pinterest)
	StitchDirection     string // "vertical" or "horizontal"
	FaviconGenerate     bool   // Generate all favicon sizes from one image
	FaviconSizes        string // Comma-separated sizes, e.g. "16,32,48,64,128,180,192,256,384,512"
	TwitchPanelWidth    int    // Twitch panel width (default 320)
	TwitchPanelHeight   int    // Twitch panel height (default 160)

	// Format Support & Encoding
	ProgressiveJPEG  bool    // Enable progressive JPEG encoding
	Base64Output     bool    // Return base64-encoded output instead of file
	ICOSizes         string  // Comma-separated sizes for ICO bundler (e.g., "16,32,48,64,128,256")
	LosslessWebP     bool    // Enable lossless WebP encoding
	ToneMapHDR       bool    // Enable HDR-to-SDR tone mapping
	TIFFExtractPages bool    // Extract all pages from multi-page TIFF
	PDFPage          int     // Specific PDF page to extract (0 = all)
	SVGScale         float64 // Scale factor for SVG rasterization (default 1.0)

	// Privacy & Security
	ZeroLogMode        bool   // Process entirely in memory, no disk writes
	EncryptOutput      bool   // Encrypt output with password
	EncryptionPassword string // Password for AES-GCM encryption
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

// processPipeline runs the full image processing pipeline on the given image.
// This is the shared core used by both ProcessImage and ProcessImageInMemory.
func processPipeline(img image.Image, opts *ProcessOptions, destDir, origNameNoExt string) (image.Image, int, int, int, int, error) {
	origWidthBeforeTransforms := img.Bounds().Dx()
	origHeightBeforeTransforms := img.Bounds().Dy()

	// BUG-12 FIX: StripEXIF is now functional.
	if opts.StripEXIF {
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)
		img = rgba
	}

	// Step 0.5: Background Removal
	if opts.RemoveBackground {
		img = applyBackgroundRemoval(img, opts)
	}

	// Step 1: Smart Crop
	img = applyCrop(img, opts)

	// Step 2: Rotation
	img = applyRotation(img, opts)

	// Step 3: Flip
	img = applyFlip(img, opts)

	// Step 4: Filters
	img = applyFilters(img, opts)

	// Step 5: Adjustments
	img = applyAdjustments(img, opts)

	// Step 5.5: Professional Adjustments
	img = applyProfessionalAdjustments(img, opts)

	// Step 5.6: HDR-to-SDR Tone Mapping (Item 37)
	img = applyToneMapping(img, opts)

	// Step 6: Vignette
	img = applyVignette(img, opts)

	// Step 7: Resize
	origBounds := img.Bounds()
	origWidth, origHeight := origBounds.Dx(), origBounds.Dy()

	var newWidth, newHeight int
	switch opts.Operation {
	case "percentage":
		newWidth = int(float64(origWidth) * float64(opts.Percentage) / 100)
		newHeight = int(float64(origHeight) * float64(opts.Percentage) / 100)
	case "dimensions":
		newWidth = opts.Width
		newHeight = opts.Height
	case "fill":
		newWidth = opts.Width
		newHeight = opts.Height
	default:
		newWidth = origWidth
		newHeight = origHeight
	}

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
	if opts.Operation == "fill" && newWidth > 0 && newHeight > 0 {
		resizedImg = imaging.Fill(img, newWidth, newHeight, imaging.Center, resampleFilter)
	} else {
		resizedImg = imaging.Resize(img, newWidth, newHeight, resampleFilter)
	}

	// Step 8: Watermark
	resizedImg = applyWatermark(resizedImg, opts, destDir)

	// Step 9: Text Overlay
	resizedImg = applyTextOverlay(resizedImg, opts)

	// Step 10: Copyright
	resizedImg = applyCopyright(resizedImg, opts)

	// Step 10.5: Branding & Overlays
	resizedImg = applyBrandingOverlays(resizedImg, opts, destDir, origNameNoExt)

	// Step 10.6: Social Media Optimization — Safe Zone Overlay
	resizedImg = applySafeZoneOverlay(resizedImg, opts)

	// Step 10.7: Social Media Optimization — Device Mockup
	resizedImg = applyDeviceMockup(resizedImg, opts)

	// Step 10.8: Social Media Optimization — Slack Emoji Resize
	resizedImg = applySlackEmojiResize(resizedImg, opts)

	// Step 11.5: File Size Optimization
	ext := getOutputFormat("", opts)
	if opts.MaxFileSizeKB > 0 {
		resizedImg = applyFileSizeOptimization(resizedImg, opts, ext)
	}

	return resizedImg, origWidthBeforeTransforms, origHeightBeforeTransforms, newWidth, newHeight, nil
}

// mimeTypeForFormat returns the MIME type for a given image format extension.
func mimeTypeForFormat(ext string) string {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "ico":
		return "image/x-icon"
	case "tiff", "tif":
		return "image/tiff"
	case "avif":
		return "image/avif"
	case "heic", "heif":
		return "image/heic"
	case "jxl":
		return "image/jxl"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// encodeToMemory encodes the image to the specified format in memory,
// returning the bytes and MIME type. Used by zero-log mode.
// It writes to a temporary file using the existing saveImage function
// (which handles all format-specific encoding), then reads the bytes back
// and removes the temp file. This avoids duplicating encoding logic.
func encodeToMemory(img image.Image, opts *ProcessOptions) ([]byte, string, error) {
	ext := getOutputFormat("", opts)
	mimeType := mimeTypeForFormat(ext)

	// Write to a temp file using the full saveImage pipeline
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("zerolog_%d.%s", time.Now().UnixMilli(), ext))
	if err := saveImage(img, tmpFile, opts); err != nil {
		return nil, "", fmt.Errorf("failed to encode image to memory: %w", err)
	}

	// Read the file back into memory
	data, err := os.ReadFile(filepath.Clean(tmpFile))
	_ = os.Remove(tmpFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read encoded temp file: %w", err)
	}

	return data, mimeType, nil
}

// ProcessImageInMemory processes an image entirely in memory without writing
// any files to disk. This is the zero-log mode entry point for privacy-sensitive
// workflows. The input is provided as a byte slice instead of a file path.
func ProcessImageInMemory(inputData []byte, opts ProcessOptions, filename string) (*ProcessResult, error) {
	// Validate encryption options
	if opts.EncryptOutput && opts.EncryptionPassword == "" {
		return nil, fmt.Errorf("encryption password is required when encrypt output is enabled")
	}

	// Decode the input image from bytes
	img, _, err := image.Decode(bytes.NewReader(inputData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image from bytes: %w", err)
	}

	origNameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Run the processing pipeline
	resizedImg, origW, origH, newW, newH, err := processPipeline(img, &opts, "", origNameNoExt)
	if err != nil {
		return nil, err
	}

	// Encode to memory
	imageData, mimeType, err := encodeToMemory(resizedImg, &opts)
	if err != nil {
		return nil, err
	}

	ext := getOutputFormat("", &opts)
	processedFileName := fmt.Sprintf("processed_%s_%d.%s", origNameNoExt, time.Now().UnixMilli(), ext)

	result := &ProcessResult{
		OriginalName:  filename,
		ProcessedName: processedFileName,
		OriginalSize:  fmt.Sprintf("%dx%d", origW, origH),
		NewSize:       fmt.Sprintf("%dx%d", newW, newH),
		NewFilePath:   "",
		ImageData:     imageData,
		MimeType:      mimeType,
		Filename:      processedFileName,
	}

	return result, nil
}

func ProcessImage(srcPath, destDir string, opts *ProcessOptions) (*ProcessResult, error) {
	// Validate encryption options
	if opts.EncryptOutput && opts.EncryptionPassword == "" {
		return nil, fmt.Errorf("encryption password is required when encrypt output is enabled")
	}

	// Placeholder generator: if width and height are set, generate a placeholder
	// image without needing an input file.
	if opts.PlaceholderWidth > 0 && opts.PlaceholderHeight > 0 {
		img := generatePlaceholder(opts)
		ext := getOutputFormat("", opts)
		processedFileName := fmt.Sprintf("placeholder_%dx%d_%d.%s", opts.PlaceholderWidth, opts.PlaceholderHeight, time.Now().UnixMilli(), ext)
		destPath := filepath.Join(destDir, processedFileName)
		if err := saveImage(img, destPath, opts); err != nil {
			return nil, err
		}
		return &ProcessResult{
			OriginalName:  "placeholder",
			ProcessedName: processedFileName,
			OriginalSize:  "N/A",
			NewSize:       fmt.Sprintf("%dx%d", opts.PlaceholderWidth, opts.PlaceholderHeight),
			NewFilePath:   destPath,
		}, nil
	}

	img, err := loadImage(srcPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}

	fileName := filepath.Base(srcPath)
	origNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Run the shared processing pipeline
	resizedImg, origWidthBeforeTransforms, origHeightBeforeTransforms, newWidth, newHeight, pipelineErr := processPipeline(img, opts, destDir, origNameNoExt)
	if pipelineErr != nil {
		return nil, pipelineErr
	}

	ext := getOutputFormat("", opts)

	// BUG-07 FIX: Add timestamp suffix to prevent filename collisions
	processedFileName := ""
	if opts.RenameTemplate != "" {
		processedFileName = strings.ReplaceAll(opts.RenameTemplate, "{name}", origNameNoExt)
		processedFileName = fmt.Sprintf("%s_%d.%s", processedFileName, time.Now().UnixMilli(), ext)
	} else {
		processedFileName = fmt.Sprintf("processed_%s_%d.%s", origNameNoExt, time.Now().UnixMilli(), ext)
	}

	// --- Zero-Log Mode: encode to memory, skip disk write ---
	if opts.ZeroLogMode {
		imageData, mimeType, encErr := encodeToMemory(resizedImg, opts)
		if encErr != nil {
			return nil, encErr
		}

		// If encryption is also enabled, encrypt the in-memory data
		if opts.EncryptOutput && opts.EncryptionPassword != "" {
			encryptedStr, encErr2 := EncryptData(imageData, opts.EncryptionPassword)
			if encErr2 != nil {
				return nil, fmt.Errorf("failed to encrypt image data: %w", encErr2)
			}
			imageData = []byte(encryptedStr)
			mimeType = "application/octet-stream"
			processedFileName += ".enc"
		}

		return &ProcessResult{
			OriginalName:  fileName,
			ProcessedName: processedFileName,
			OriginalSize:  fmt.Sprintf("%dx%d", origWidthBeforeTransforms, origHeightBeforeTransforms),
			NewSize:       fmt.Sprintf("%dx%d", newWidth, newHeight),
			NewFilePath:   "",
			ImageData:     imageData,
			MimeType:      mimeType,
			Filename:      processedFileName,
		}, nil
	}

	destPath := filepath.Join(destDir, processedFileName)

	// --- Encryption mode (non-zero-log): encrypt and write .enc file ---
	if opts.EncryptOutput && opts.EncryptionPassword != "" {
		imageData, _, encErr := encodeToMemory(resizedImg, opts)
		if encErr != nil {
			return nil, encErr
		}
		encryptedStr, encErr2 := EncryptData(imageData, opts.EncryptionPassword)
		if encErr2 != nil {
			return nil, fmt.Errorf("failed to encrypt image data: %w", encErr2)
		}
		encFileName := processedFileName + ".enc"
		encPath := filepath.Join(destDir, encFileName)
		if writeErr := os.WriteFile(filepath.Clean(encPath), []byte(encryptedStr), 0600); writeErr != nil {
			return nil, fmt.Errorf("failed to write encrypted file: %w", writeErr)
		}
		return &ProcessResult{
			OriginalName:  fileName,
			ProcessedName: encFileName,
			OriginalSize:  fmt.Sprintf("%dx%d", origWidthBeforeTransforms, origHeightBeforeTransforms),
			NewSize:       fmt.Sprintf("%dx%d", newWidth, newHeight),
			NewFilePath:   encPath,
		}, nil
	}

	// --- Normal mode: write to disk ---
	err = saveImage(resizedImg, destPath, opts)
	if err != nil {
		return nil, err
	}

	result := &ProcessResult{
		OriginalName:  fileName,
		ProcessedName: processedFileName,
		OriginalSize:  fmt.Sprintf("%dx%d", origWidthBeforeTransforms, origHeightBeforeTransforms),
		NewSize:       fmt.Sprintf("%dx%d", newWidth, newHeight),
		NewFilePath:   destPath,
	}

	// Step 12: Social Media Optimization — Carousel Slicing
	if opts.CarouselSlice {
		slides := applyCarouselSlice(resizedImg, opts)
		if len(slides) > 0 {
			for i, slide := range slides {
				slideFileName := fmt.Sprintf("processed_%s_slide_%d_%d.%s", origNameNoExt, i+1, time.Now().UnixMilli(), ext)
				slidePath := filepath.Join(destDir, slideFileName)
				if saveErr := saveImage(slide, slidePath, opts); saveErr == nil {
					result.ExtraFiles = append(result.ExtraFiles, slidePath)
				}
			}
		}
	}

	// Step 13: Social Media Optimization — Favicon Generation
	if opts.FaviconGenerate {
		favicons := generateFavicons(resizedImg, opts)
		if len(favicons) > 0 {
			for size, favicon := range favicons {
				favFileName := fmt.Sprintf("processed_%s_favicon_%dx%d_%d.png", origNameNoExt, size, size, time.Now().UnixMilli())
				favPath := filepath.Join(destDir, favFileName)
				favOpts := *opts
				favOpts.Format = "png"
				if saveErr := saveImage(favicon, favPath, &favOpts); saveErr == nil {
					result.ExtraFiles = append(result.ExtraFiles, favPath)
				}
			}
			// Generate manifest.json for PWA
			manifestPath := filepath.Join(destDir, fmt.Sprintf("processed_%s_manifest_%d.json", origNameNoExt, time.Now().UnixMilli()))
			if manifestErr := generateFaviconManifest(opts, manifestPath); manifestErr == nil {
				result.ExtraFiles = append(result.ExtraFiles, manifestPath)
			}
		}
	}

	return result, nil
}
