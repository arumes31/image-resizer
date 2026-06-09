package processor

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/jung-kurt/gofpdf"
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
	"bilinear": imaging.Box,
	"bicubic":  imaging.CatmullRom,
	"lanczos":  imaging.Lanczos,
}

type ProcessResult struct {
	OriginalName string
	ProcessedName string
	OriginalSize string
	NewSize      string
	NewFilePath  string
}

type ProcessOptions struct {
	Operation    string
	Percentage   int
	Width        int
	Height       int
	Quality      int
	Method       string
	Format       string
	Rotation     int    // 0, 90, 180, 270
	Flip         string // "h", "v", "both", ""
	Filters      []string // "grayscale", "sepia", "invert", "blur", "sharpen", "pixelate", "noir", "vivid"
	WatermarkPath string
	WatermarkPos  string // "center", "top-left", etc.
	TextOverlay   string
	TextColor     string // hex
	TextSize      float64
	StripEXIF     bool
	Copyright     string
	Brightness    float64 // -100 to 100
	Contrast      float64 // -100 to 100
	Saturation    float64 // -100 to 100
	Pixelate      int     // 0 (off) to 100
	Crop          string  // "1:1", "16:9", "4:3", "none"
	Vignette      bool
	RenameTemplate string
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

func ProcessImage(srcPath, destDir string, opts ProcessOptions) (*ProcessResult, error) {
	img, err := imaging.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
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
	if opts.Flip == "h" {
		img = imaging.FlipH(img)
	} else if opts.Flip == "v" {
		img = imaging.FlipV(img)
	} else if opts.Flip == "both" {
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
			img = imaging.AdjustContrast(img, 10)
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
				img = imaging.Resize(img, w/pxSize, h/pxSize, imaging.NearestNeighbor)
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

	origBounds := img.Bounds()
	origWidth, origHeight := origBounds.Dx(), origBounds.Dy()

	var newWidth, newHeight int
	if opts.Operation == "percentage" {
		newWidth = int(float64(origWidth) * float64(opts.Percentage) / 100)
		newHeight = int(float64(origHeight) * float64(opts.Percentage) / 100)
	} else {
		newWidth = opts.Width
		newHeight = opts.Height
	}

	resampleFilter := imaging.Lanczos
	if f, ok := ResizeMethods[strings.ToLower(opts.Method)]; ok {
		resampleFilter = f
	}

	var resizedImg image.Image
	resizedImg = imaging.Resize(img, newWidth, newHeight, resampleFilter)

	// 4. Watermarking
	if opts.WatermarkPath != "" {
		wm, err := imaging.Open(opts.WatermarkPath)
		if err == nil {
			resizedImg = imaging.OverlayCenter(resizedImg, wm, 0.5)
		}
	}

	// 5. Text Overlay
	if opts.TextOverlay != "" {
		dst := image.NewRGBA(resizedImg.Bounds())
		draw.Draw(dst, dst.Bounds(), resizedImg, image.Point{}, draw.Src)

		fontBytes, err := os.ReadFile("C:\\Windows\\Fonts\\arial.ttf")
		if err == nil {
			f, err := freetype.ParseFont(fontBytes)
			if err == nil {
				fg := image.NewUniform(color.White)
				c := freetype.NewContext()
				c.SetDPI(72)
				c.SetFont(f)
				c.SetFontSize(24)
				c.SetClip(dst.Bounds())
				c.SetDst(dst)
				c.SetSrc(fg)
				c.SetHinting(font.HintingFull)

				pt := freetype.Pt(10, dst.Bounds().Dy()-10)
				_, _ = c.DrawString(opts.TextOverlay, pt)
				resizedImg = dst
			}
		}
	}

	fileName := filepath.Base(srcPath)
	origNameNoExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	ext := strings.ToLower(opts.Format)
	if ext == "" || ext == "pdf" {
		ext = "jpg" // PDF intermediate is JPG
	}
	
	processedFileName := ""
	if opts.RenameTemplate != "" {
		processedFileName = strings.ReplaceAll(opts.RenameTemplate, "{name}", origNameNoExt)
		processedFileName = fmt.Sprintf("%s.%s", processedFileName, ext)
	} else {
		processedFileName = fmt.Sprintf("processed_%s.%s", origNameNoExt, ext)
	}
	
	destPath := filepath.Join(destDir, processedFileName)

	err = imaging.Save(resizedImg, destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to save image: %w", err)
	}

	return &ProcessResult{
		OriginalName: fileName,
		ProcessedName: processedFileName,
		OriginalSize: fmt.Sprintf("%dx%d", origWidth, origHeight),
		NewSize:      fmt.Sprintf("%dx%d", newWidth, newHeight),
		NewFilePath:  destPath,
	}, nil
}
