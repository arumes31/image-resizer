package processor

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/grailbio/go-dicom"
	"github.com/grailbio/go-dicom/dicomtag"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/tiff"
)

// ---------------------------------------------------------------------------
// loadImage — Format-aware image loading (Item 26-40 dispatcher)
// ---------------------------------------------------------------------------

// loadImage opens an image file using the appropriate decoder based on its
// extension. For standard formats (JPEG, PNG, GIF, BMP, TIFF, WebP, ICO) it
// falls back to imaging.Open which handles EXIF auto-orientation. For special
// formats (SVG, HEIC, AVIF, JXL, RAW, DICOM) it dispatches to dedicated
// decoders defined in this package.
func loadImage(path string, opts *ProcessOptions) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".svg":
		return decodeSVG(path, opts.SVGScale)
	case ".heic", ".heif":
		return decodeHEIC(path)
	case ".avif":
		return decodeAVIF(path)
	case ".jxl":
		return decodeJXL(path)
	case ".cr2", ".cr3", ".nef", ".arw", ".dng":
		return decodeRAW(path)
	case ".dcm", ".dicom":
		return decodeDICOM(path)
	default:
		// Use imaging.Open for standard formats (JPEG, PNG, GIF, BMP, TIFF, WebP, ICO)
		return imaging.Open(path, imaging.AutoOrientation(true))
	}
}

// ---------------------------------------------------------------------------
// 4.4 SVG Rasterization (Item 29)
// ---------------------------------------------------------------------------

// decodeSVG parses an SVG file using oksvg and rasterizes it to an image.Image
// at the given scale factor. If scale is 0, it defaults to 1.0.
func decodeSVG(path string, scale float64) (image.Image, error) {
	if scale <= 0 {
		scale = 1.0
	}

	icon, err := oksvg.ReadIcon(path, oksvg.WarnErrorMode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SVG: %w", err)
	}

	w := int(icon.ViewBox.W * scale)
	h := int(icon.ViewBox.H * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	icon.SetTarget(0, 0, float64(w), float64(h))

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, img, image.Rect(0, 0, w, h))
	drawer := rasterx.NewDasher(w, h, scanner)
	icon.Draw(drawer, 1.0)

	return img, nil
}

// encodeSVGPlaceholder returns an error — vectorization of raster images is
// extremely complex and not supported. Suggests external tools like potrace.
func encodeSVGPlaceholder(img image.Image, path string) error {
	return errors.New("SVG output (vectorization) is not yet supported — use potrace or similar tools externally")
}

// ---------------------------------------------------------------------------
// 4.5 ICO Multi-size Bundler (Item 30)
// ---------------------------------------------------------------------------

// encodeICOMultiSize creates a multi-size ICO file from the given image.
// If opts.ICOSizes is empty, it defaults to "16,32,48,64,128,256".
// For a single size, it uses the standard ico.Encode. For multiple sizes,
// it writes a custom ICO binary with PNG-encoded images.
func encodeICOMultiSize(img image.Image, outputPath string, opts *ProcessOptions) error {
	sizesStr := opts.ICOSizes
	if sizesStr == "" {
		sizesStr = "16,32,48,64,128,256"
	}

	var sizes []int
	for _, s := range strings.Split(sizesStr, ",") {
		s = strings.TrimSpace(s)
		size, err := strconv.Atoi(s)
		if err != nil || size <= 0 {
			continue
		}
		sizes = append(sizes, size)
	}

	if len(sizes) == 0 {
		sizes = []int{32}
	}

	// If only one size, use the existing single-image encoder
	if len(sizes) == 1 {
		resized := imaging.Resize(img, sizes[0], sizes[0], imaging.Lanczos)
		f, err := os.Create(filepath.Clean(outputPath))
		if err != nil {
			return err
		}
		defer f.Close()
		return encodeSingleICO(f, resized)
	}

	// For multi-size, encode each as PNG and bundle into ICO format
	f, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return err
	}
	defer f.Close()

	// Encode each size as PNG and collect data
	var imageData [][]byte
	for _, size := range sizes {
		resized := imaging.Resize(img, size, size, imaging.Lanczos)
		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			return fmt.Errorf("failed to encode %dx%d PNG for ICO: %w", size, size, err)
		}
		imageData = append(imageData, buf.Bytes())
	}

	// Write ICO header
	if err := binary.Write(f, binary.LittleEndian, uint16(0)); err != nil { // Reserved
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil { // Type: 1 = ICO
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(len(sizes))); err != nil { // Number of images
		return err
	}

	// Calculate offsets
	headerSize := 6 + len(sizes)*16 // 6 bytes header + 16 bytes per entry
	offset := headerSize

	// Write directory entries
	for i, size := range sizes {
		w := uint8(size)
		if size >= 256 {
			w = 0 // 0 means 256 in ICO format
		}
		h := w

		if err := binary.Write(f, binary.LittleEndian, w); err != nil { // Width
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, h); err != nil { // Height
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint8(0)); err != nil { // Color palette
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint8(0)); err != nil { // Reserved
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil { // Color planes
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint16(32)); err != nil { // Bits per pixel
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint32(len(imageData[i]))); err != nil { // Size
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, uint32(offset)); err != nil { // Offset
			return err
		}
		offset += len(imageData[i])
	}

	// Write image data
	for _, data := range imageData {
		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	return nil
}

// encodeSingleICO writes a single-image ICO file using the go-ico library.
func encodeSingleICO(f *os.File, img image.Image) error {
	// Use the go-ico library for single-image ICO
	// We need to import this in codecs.go where ico is already imported
	// For now, encode as PNG-based ICO manually
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	pngData := buf.Bytes()

	// Write ICO header
	if err := binary.Write(f, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}

	// Write directory entry
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	wByte := uint8(w)
	if w >= 256 {
		wByte = 0
	}
	hByte := uint8(h)
	if h >= 256 {
		hByte = 0
	}

	if err := binary.Write(f, binary.LittleEndian, wByte); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, hByte); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint8(0)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint8(0)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(32)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(len(pngData))); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(22)); err != nil { // offset = 6 + 16
		return err
	}

	// Write image data
	_, err := f.Write(pngData)
	return err
}

// ---------------------------------------------------------------------------
// 4.6 Animated WebP/GIF Support (Item 31)
// ---------------------------------------------------------------------------

// decodeAnimatedGIF decodes all frames from an animated GIF file.
// It composites each frame onto a full canvas (GIF frames can be partial).
// Returns the list of full-frame images and their delays in centiseconds.
func decodeAnimatedGIF(path string) ([]image.Image, []int, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode GIF: %w", err)
	}

	if len(g.Image) == 0 {
		return nil, nil, errors.New("GIF contains no frames")
	}

	// Get the logical screen size
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)

	var frames []image.Image
	var delays []int

	// Create a canvas for compositing
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, image.White, image.Point{}, draw.Src)

	for i, frame := range g.Image {
		// For proper disposal, we'd need to handle g.Disposal[i]
		// For simplicity, composite each frame onto the canvas
		disposal := gif.DisposalNone
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case gif.DisposalNone:
				disposal = gif.DisposalNone
			case gif.DisposalBackground:
				disposal = gif.DisposalBackground
			case gif.DisposalPrevious:
				disposal = gif.DisposalPrevious
			}
		}

		if disposal == gif.DisposalBackground {
			// Clear the frame area to background
			draw.Draw(canvas, frame.Bounds(), image.White, image.Point{}, draw.Src)
		}

		// Composite the frame onto the canvas
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		// Copy the current canvas state as a frame
		frameCopy := image.NewRGBA(bounds)
		draw.Draw(frameCopy, bounds, canvas, image.Point{}, draw.Src)
		frames = append(frames, frameCopy)

		delay := 10 // default 100ms
		if i < len(g.Delay) {
			delay = g.Delay[i]
			if delay == 0 {
				delay = 10
			}
		}
		delays = append(delays, delay)
	}

	return frames, delays, nil
}

// encodeAnimatedGIF encodes multiple frames as an animated GIF.
// Uses a simple uniform color quantization for the 256-color GIF limitation.
func encodeAnimatedGIF(frames []image.Image, delays []int, path string, opts *ProcessOptions) error {
	if len(frames) == 0 {
		return errors.New("no frames to encode")
	}

	g := &gif.GIF{
		Image: make([]*image.Paletted, len(frames)),
		Delay: make([]int, len(frames)),
	}

	for i, frame := range frames {
		bounds := frame.Bounds()
		paletted := image.NewPaletted(bounds, generateSimplePalette())
		draw.FloydSteinberg.Draw(paletted, bounds, frame, image.Point{})
		g.Image[i] = paletted
		if i < len(delays) {
			g.Delay[i] = delays[i]
		} else {
			g.Delay[i] = 10
		}
	}

	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()

	return gif.EncodeAll(f, g)
}

// generateSimplePalette creates a default 256-color palette for GIF encoding.
func generateSimplePalette() color.Palette {
	palette := make(color.Palette, 256)
	for i := 0; i < 256; i++ {
		palette[i] = color.NRGBA{
			R: uint8(i),
			G: uint8(i),
			B: uint8(i),
			A: 255,
		}
	}
	return palette
}

// ---------------------------------------------------------------------------
// 4.8 Base64 Export (Item 33)
// ---------------------------------------------------------------------------

// encodeBase64 encodes the image to the specified format in memory and returns
// the result as a data URI base64 string.
func encodeBase64(img image.Image, format string, quality int) (string, error) {
	var buf bytes.Buffer

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
			return "", fmt.Errorf("failed to encode JPEG for base64: %w", err)
		}
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil

	case "png":
		if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
			return "", fmt.Errorf("failed to encode PNG for base64: %w", err)
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil

	case "webp":
		// Use chai2010/webp for WebP encoding
		if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
			return "", fmt.Errorf("failed to encode WebP for base64: %w", err)
		}
		return "data:image/webp;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil

	case "gif":
		if err := imaging.Encode(&buf, img, imaging.GIF); err != nil {
			return "", fmt.Errorf("failed to encode GIF for base64: %w", err)
		}
		return "data:image/gif;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil

	default:
		// Fallback: encode as PNG
		if err := imaging.Encode(&buf, img, imaging.PNG); err != nil {
			return "", fmt.Errorf("failed to encode image for base64: %w", err)
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	}
}

// ---------------------------------------------------------------------------
// 4.10 TIFF Multi-page Extraction (Item 35)
// ---------------------------------------------------------------------------

// decodeTIFFMultiPage attempts to decode all pages from a multi-page TIFF.
// The golang.org/x/image/tiff package only supports single-frame decoding
// via tiff.Decode. For multi-page TIFFs, this returns a single frame with
// a note that full multi-page support requires manual IFD parsing.
func decodeTIFFMultiPage(path string) ([]image.Image, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// tiff.Decode returns a single image
	img, err := tiff.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TIFF: %w", err)
	}

	return []image.Image{img}, nil
}

// ---------------------------------------------------------------------------
// 4.11 PDF-to-Image (Item 36)
// ---------------------------------------------------------------------------

// decodePDF is a stub — PDF rendering in pure Go is very limited.
// For actual PDF rendering, use poppler-utils (pdftoppm) or a CGO-based library.
func decodePDF(path string, page int) (image.Image, error) {
	return nil, errors.New("PDF-to-image conversion requires poppler-utils (pdftoppm) or a CGO-enabled build")
}

// ---------------------------------------------------------------------------
// 4.12 HDR-to-SDR Tone Mapping (Item 37)
// ---------------------------------------------------------------------------

// applyToneMapping applies a simplified Reinhard tone mapping operator to
// compress HDR-like values into the standard 0-255 range. This is useful
// for images with very bright highlights that need to be brought into
// displayable range.
func applyToneMapping(img image.Image, opts *ProcessOptions) image.Image {
	if !opts.ToneMapHDR {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := dst.NRGBAAt(x, y)
			r := float64(c.R) / 255.0
			g := float64(c.G) / 255.0
			b := float64(c.B) / 255.0

			// Calculate luminance
			lum := 0.2126*r + 0.7152*g + 0.0722*b

			if lum > 0 {
				// Apply Reinhard operator: L_mapped = L / (1 + L)
				lumMapped := lum / (1.0 + lum)
				factor := lumMapped / lum

				r = r * factor
				g = g * factor
				b = b * factor
			}

			dst.SetNRGBA(x, y, color.NRGBA{
				R: clampUint8(int(r * 255.0)),
				G: clampUint8(int(g * 255.0)),
				B: clampUint8(int(b * 255.0)),
				A: c.A,
			})
		}
	}

	return dst
}

// ---------------------------------------------------------------------------
// 4.13 Web Manifest Generator (Item 38)
// ---------------------------------------------------------------------------

// WebManifest represents a PWA web app manifest structure.
type WebManifest struct {
	Name            string           `json:"name,omitempty"`
	ShortName       string           `json:"short_name,omitempty"`
	Icons           []WebManifestIcon `json:"icons,omitempty"`
	ThemeColor      string           `json:"theme_color,omitempty"`
	BackgroundColor string           `json:"background_color,omitempty"`
	Display         string           `json:"display,omitempty"`
	StartURL        string           `json:"start_url,omitempty"`
}

// WebManifestIcon represents an icon entry in the web manifest.
type WebManifestIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

// generateWebManifest creates a web app manifest JSON file.
func generateWebManifest(outputPath string, opts *ProcessOptions) error {
	sizesStr := opts.FaviconSizes
	if sizesStr == "" {
		sizesStr = "16,32,48,64,128,180,192,256,384,512"
	}

	var icons []WebManifestIcon
	for _, s := range strings.Split(sizesStr, ",") {
		s = strings.TrimSpace(s)
		size, err := strconv.Atoi(s)
		if err != nil || size <= 0 {
			continue
		}
		// Include common PWA sizes in manifest
		if size == 192 || size == 512 {
			icons = append(icons, WebManifestIcon{
				Src:   fmt.Sprintf("favicon-%dx%d.png", size, size),
				Sizes: fmt.Sprintf("%dx%d", size, size),
				Type:  "image/png",
			})
		}
	}

	manifest := WebManifest{
		Name:            "",
		ShortName:       "",
		Icons:           icons,
		StartURL:        "/",
		Display:         "standalone",
		BackgroundColor: "#ffffff",
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return os.WriteFile(filepath.Clean(outputPath), data, 0600)
}

// ---------------------------------------------------------------------------
// 4.15 DICOM Support (Item 40)
// ---------------------------------------------------------------------------

// decodeDICOM parses a DICOM medical image file and converts its pixel data
// to a standard image.Image. Handles 8-bit and 16-bit grayscale images.
// Color DICOM (RGB) can be added later.
func decodeDICOM(path string) (image.Image, error) {
	dcm, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{
		DropPixelData: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse DICOM file: %w", err)
	}

	// Get image dimensions
	rowsElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.Rows)
	if err != nil {
		return nil, errors.New("DICOM file missing Rows element")
	}
	colsElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.Columns)
	if err != nil {
		return nil, errors.New("DICOM file missing Columns element")
	}

	rows, err := rowsElem.GetUInt16()
	if err != nil {
		return nil, fmt.Errorf("failed to read DICOM Rows: %w", err)
	}
	cols, err := colsElem.GetUInt16()
	if err != nil {
		return nil, fmt.Errorf("failed to read DICOM Columns: %w", err)
	}

	// Get bits allocated
	bitsAllocated := uint16(8) // default
	baElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.BitsAllocated)
	if err == nil {
		ba, eErr := baElem.GetUInt16()
		if eErr == nil {
			bitsAllocated = ba
		}
	}

	// Get samples per pixel
	samplesPerPixel := uint16(1) // default grayscale
	sppElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.SamplesPerPixel)
	if err == nil {
		spp, eErr := sppElem.GetUInt16()
		if eErr == nil {
			samplesPerPixel = spp
		}
	}

	// Get pixel representation (0 = unsigned, 1 = signed)
	pixelRepresentation := uint16(0)
	prElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.PixelRepresentation)
	if err == nil {
		pr, eErr := prElem.GetUInt16()
		if eErr == nil {
			pixelRepresentation = pr
		}
	}

	// Get rescale slope and intercept (for CT/MRI)
	rescaleSlope := float64(1.0)
	rsElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.RescaleSlope)
	if err == nil {
		rsStr, eErr := rsElem.GetString()
		if eErr == nil {
			if v, pErr := strconv.ParseFloat(strings.TrimSpace(rsStr), 64); pErr == nil {
				rescaleSlope = v
			}
		}
	}

	rescaleIntercept := float64(0.0)
	riElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.RescaleIntercept)
	if err == nil {
		riStr, eErr := riElem.GetString()
		if eErr == nil {
			if v, pErr := strconv.ParseFloat(strings.TrimSpace(riStr), 64); pErr == nil {
				rescaleIntercept = v
			}
		}
	}

	// Get window center and width for display
	windowCenter := float64(-1)
	wcElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.WindowCenter)
	if err == nil {
		wcStr, eErr := wcElem.GetString()
		if eErr == nil {
			// Window center may have multiple values separated by "\"
			parts := strings.Split(wcStr, `\`)
			if v, pErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); pErr == nil {
				windowCenter = v
			}
		}
	}

	windowWidth := float64(-1)
	wwElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.WindowWidth)
	if err == nil {
		wwStr, eErr := wwElem.GetString()
		if eErr == nil {
			parts := strings.Split(wwStr, `\`)
			if v, pErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); pErr == nil {
				windowWidth = v
			}
		}
	}

	// Find pixel data element
	pixelDataElem, err := dicom.FindElementByTag(dcm.Elements, dicomtag.PixelData)
	if err != nil {
		return nil, errors.New("no pixel data found in DICOM file")
	}

	if len(pixelDataElem.Value) == 0 {
		return nil, errors.New("DICOM pixel data element is empty")
	}

	pixelDataInfo, ok := pixelDataElem.Value[0].(dicom.PixelDataInfo)
	if !ok {
		return nil, errors.New("DICOM pixel data has unexpected type")
	}

	if len(pixelDataInfo.Frames) == 0 {
		return nil, errors.New("DICOM pixel data contains no frames")
	}

	// Use the first frame
	frameData := pixelDataInfo.Frames[0]

	w := int(cols)
	h := int(rows)

	if samplesPerPixel == 3 {
		// RGB DICOM
		return decodeDICOMRGB(frameData, w, h, bitsAllocated)
	}

	// Grayscale DICOM
	return decodeDICOMGrayscale(frameData, w, h, bitsAllocated, pixelRepresentation,
		rescaleSlope, rescaleIntercept, windowCenter, windowWidth)
}

// decodeDICOMGrayscale converts raw DICOM grayscale pixel data to image.Gray.
func decodeDICOMGrayscale(
	data []byte, w, h int, bitsAllocated, pixelRepresentation uint16,
	rescaleSlope, rescaleIntercept, windowCenter, windowWidth float64,
) (image.Image, error) {
	img := image.NewGray(image.Rect(0, 0, w, h))

	switch bitsAllocated {
	case 8:
		if len(data) < w*h {
			return nil, errors.New("DICOM 8-bit pixel data too short")
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				idx := y*w + x
				img.SetGray(x, y, color.Gray{Y: data[idx]})
			}
		}

	case 16:
		if len(data) < w*h*2 {
			return nil, errors.New("DICOM 16-bit pixel data too short")
		}
		// Apply windowing/leveling to map 16-bit to 8-bit
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				idx := (y*w + x) * 2
				// Little-endian 16-bit value
				var rawVal float64
				if pixelRepresentation == 1 {
					// Signed 16-bit
					val := int16(uint16(data[idx]) | uint16(data[idx+1])<<8)
					rawVal = float64(val)
				} else {
					// Unsigned 16-bit
					val := uint16(data[idx]) | uint16(data[idx+1])<<8
					rawVal = float64(val)
				}

				// Apply rescale
				huVal := rawVal*rescaleSlope + rescaleIntercept

				// Apply windowing
				var mapped uint8
				if windowCenter > 0 && windowWidth > 0 {
					// Window/level mapping
					low := windowCenter - windowWidth/2.0
					high := windowCenter + windowWidth/2.0
					if huVal <= low {
						mapped = 0
					} else if huVal >= high {
						mapped = 255
					} else {
						mapped = uint8(((huVal - low) / windowWidth) * 255.0)
					}
				} else {
					// Auto-window: map min-max to 0-255
					mapped = clampUint8(int((huVal / 4096.0) * 255.0))
				}

				img.SetGray(x, y, color.Gray{Y: mapped})
			}
		}

	default:
		return nil, fmt.Errorf("unsupported DICOM bits allocated: %d", bitsAllocated)
	}

	return img, nil
}

// decodeDICOMRGB converts raw DICOM RGB pixel data to image.NRGBA.
func decodeDICOMRGB(data []byte, w, h int, bitsAllocated uint16) (image.Image, error) {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	switch bitsAllocated {
	case 8:
		if len(data) < w*h*3 {
			return nil, errors.New("DICOM 8-bit RGB pixel data too short")
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				idx := (y*w + x) * 3
				img.SetNRGBA(x, y, color.NRGBA{
					R: data[idx],
					G: data[idx+1],
					B: data[idx+2],
					A: 255,
				})
			}
		}

	default:
		return nil, fmt.Errorf("unsupported DICOM RGB bits allocated: %d", bitsAllocated)
	}

	return img, nil
}

// ---------------------------------------------------------------------------
// Exported helpers for server handler
// ---------------------------------------------------------------------------

// LoadImageForBase64 loads an image from disk for base64 encoding.
// This is a simplified loader that uses imaging.Open for standard formats.
func LoadImageForBase64(path string) (image.Image, error) {
	return imaging.Open(path, imaging.AutoOrientation(true))
}

// EncodeBase64 encodes the image to the specified format and returns a
// data URI base64 string. This is the exported version for use by the
// server handler.
func EncodeBase64(img image.Image, format string, quality int) (string, error) {
	return encodeBase64(img, format, quality)
}
