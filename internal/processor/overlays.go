package processor

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/ean"
	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
)

// applyWatermark overlays a watermark image onto the source image.
func applyWatermark(img image.Image, opts *ProcessOptions, uploadDir string) image.Image {
	if opts.WatermarkPath == "" {
		return img
	}

	wm, wErr := imaging.Open(opts.WatermarkPath)
	if wErr != nil {
		fmt.Printf("Warning: failed to open watermark %s: %v\n", opts.WatermarkPath, wErr)
		return img
	}

	// Use configurable opacity; default to 0.5 for backward compatibility
	opacity := opts.WatermarkOpacity
	if opacity == 0 {
		opacity = 0.5
	}

	pos := strings.ToLower(opts.WatermarkPos)
	if pos == "" {
		pos = "center"
	}

	switch pos {
	case "top-left":
		bounds := img.Bounds()
		offset := image.Pt(10, 10)
		dst := image.NewNRGBA(bounds)
		draw.Draw(dst, bounds, img, image.Point{}, draw.Src)
		draw.Draw(dst, bounds, wm, offset, draw.Over)
		return dst
	case "top-right":
		bounds := img.Bounds()
		offset := image.Pt(bounds.Dx()-wm.Bounds().Dx()-10, 10)
		dst := image.NewNRGBA(bounds)
		draw.Draw(dst, bounds, img, image.Point{}, draw.Src)
		draw.Draw(dst, bounds, wm, offset, draw.Over)
		return dst
	case "bottom-left":
		bounds := img.Bounds()
		offset := image.Pt(10, bounds.Dy()-wm.Bounds().Dy()-10)
		dst := image.NewNRGBA(bounds)
		draw.Draw(dst, bounds, img, image.Point{}, draw.Src)
		draw.Draw(dst, bounds, wm, offset, draw.Over)
		return dst
	case "bottom-right":
		bounds := img.Bounds()
		offset := image.Pt(bounds.Dx()-wm.Bounds().Dx()-10, bounds.Dy()-wm.Bounds().Dy()-10)
		dst := image.NewNRGBA(bounds)
		draw.Draw(dst, bounds, img, image.Point{}, draw.Src)
		draw.Draw(dst, bounds, wm, offset, draw.Over)
		return dst
	case "tile":
		return applyTiledWatermark(img, opts, uploadDir)
	default:
		// Center overlay with configurable opacity
		return imaging.OverlayCenter(img, wm, opacity)
	}
}

// applyTextOverlay renders user-specified text onto the image using freetype.
func applyTextOverlay(img image.Image, opts *ProcessOptions) image.Image {
	if opts.TextOverlay == "" {
		return img
	}

	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

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
				img = dst
			} else {
				fmt.Printf("Warning: failed to parse font: %v\n", pErr)
			}
		} else {
			fmt.Printf("Warning: failed to load font at %s: %v\n", fontPath, fErr)
		}
	} else {
		fmt.Println("Warning: No suitable font found for text overlay")
	}

	return img
}

// applyCopyright adds a small copyright notice at the bottom-right of the image.
// BUG-11 FIX: Copyright field was defined but never used.
func applyCopyright(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Copyright == "" {
		return img
	}

	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

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
				img = dst
			}
		}
	}

	return img
}

// ---------------------------------------------------------------------------
// Branding & Overlays (Items 51-60)
// ---------------------------------------------------------------------------

// applyBrandingOverlays applies all branding overlay functions in order.
// Order matters: overlays positioned relative to original dimensions come first,
// then canvas-expanding operations (border, shadow) come last.
func applyBrandingOverlays(img image.Image, opts *ProcessOptions, uploadDir string, filename string) image.Image {
	// 1. Dynamic Text Watermark
	if opts.WatermarkTemplate != "" {
		img = applyDynamicTextWatermark(img, opts, filename)
	}
	// 2. Tiled Watermark
	if opts.WatermarkTile {
		img = applyTiledWatermark(img, opts, uploadDir)
	}
	// 3. QR Code Overlay
	if opts.QRCodeText != "" {
		img = applyQRCodeOverlay(img, opts)
	}
	// 4. Barcode Overlay
	if opts.BarcodeText != "" {
		img = applyBarcodeOverlay(img, opts)
	}
	// 5. Signature Stamp
	if opts.SignaturePath != "" {
		img = applySignature(img, opts, uploadDir)
	}
	// 6. Rounded Corners (before canvas-expanding ops)
	if opts.RoundedCorners > 0 {
		img = applyRoundedCorners(img, opts)
	}
	// 7. Drop Shadow (expands canvas)
	if opts.DropShadowOffset > 0 {
		img = applyDropShadow(img, opts)
	}
	// 8. Border (expands canvas)
	if opts.BorderWidth > 0 {
		img = applyBorder(img, opts)
	}
	// 9. Steganography (imperceptible, order doesn't matter)
	if opts.SteganographyText != "" {
		img = applySteganography(img, opts)
	}
	return img
}

// ---------------------------------------------------------------------------
// 3.1 Dynamic Text Watermarks (Item 51)
// ---------------------------------------------------------------------------

// applyDynamicTextWatermark replaces template placeholders and renders text.
func applyDynamicTextWatermark(img image.Image, opts *ProcessOptions, filename string) image.Image {
	if opts.WatermarkTemplate == "" {
		return img
	}

	now := time.Now()
	bounds := img.Bounds()
	text := opts.WatermarkTemplate
	text = strings.ReplaceAll(text, "{filename}", filename)
	text = strings.ReplaceAll(text, "{date}", now.Format("2006-01-02"))
	text = strings.ReplaceAll(text, "{time}", now.Format("15:04:05"))
	text = strings.ReplaceAll(text, "{year}", now.Format("2006"))
	text = strings.ReplaceAll(text, "{camera}", "Unknown")
	text = strings.ReplaceAll(text, "{width}", fmt.Sprintf("%d", bounds.Dx()))
	text = strings.ReplaceAll(text, "{height}", fmt.Sprintf("%d", bounds.Dy()))

	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	fontPath := os.Getenv("FONT_PATH")
	if fontPath == "" {
		fontPath = getPlatformFontPath()
	}
	if fontPath == "" {
		fmt.Println("Warning: No suitable font found for dynamic text watermark")
		return img
	}

	fontBytes, fErr := os.ReadFile(filepath.Clean(fontPath)) // #nosec G304
	if fErr != nil {
		fmt.Printf("Warning: failed to load font for dynamic watermark: %v\n", fErr)
		return img
	}

	f, pErr := freetype.ParseFont(fontBytes)
	if pErr != nil {
		fmt.Printf("Warning: failed to parse font for dynamic watermark: %v\n", pErr)
		return img
	}

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

	// Position at bottom-left with padding
	pt := freetype.Pt(10, dst.Bounds().Dy()-10)
	if _, dErr := c.DrawString(text, pt); dErr != nil {
		fmt.Printf("Warning: failed to draw dynamic text watermark: %v\n", dErr)
	}

	return dst
}

// ---------------------------------------------------------------------------
// 3.2 Tiled Watermarks (Item 52)
// ---------------------------------------------------------------------------

// applyTiledWatermark tiles a watermark image across the entire image with
// configurable spacing and opacity, optionally rotated 30° for a diagonal effect.
func applyTiledWatermark(img image.Image, opts *ProcessOptions, uploadDir string) image.Image {
	if opts.WatermarkPath == "" {
		return img
	}

	wm, wErr := imaging.Open(opts.WatermarkPath)
	if wErr != nil {
		fmt.Printf("Warning: failed to open watermark for tiling %s: %v\n", opts.WatermarkPath, wErr)
		return img
	}

	// Scale watermark tile to max 100px wide
	tileW := wm.Bounds().Dx()
	if tileW > 100 {
		wm = imaging.Resize(wm, 100, 0, imaging.Lanczos)
	}

	opacity := opts.WatermarkOpacity
	if opacity == 0 {
		opacity = 0.3
	}

	spacing := opts.WatermarkTileSpacing
	if spacing == 0 {
		spacing = 50
	}

	// Apply opacity to the watermark tile
	tileBounds := wm.Bounds()
	opaqueTile := image.NewNRGBA(tileBounds)
	for y := tileBounds.Min.Y; y < tileBounds.Max.Y; y++ {
		for x := tileBounds.Min.X; x < tileBounds.Max.X; x++ {
			cR, cG, cB, cA := wm.At(x, y).RGBA()
			newA := uint8(float64(cA>>8) * opacity)
			if newA == 0 {
				continue
			}
			opaqueTile.SetNRGBA(x, y, color.NRGBA{
				R: uint8(cR >> 8),
				G: uint8(cG >> 8),
				B: uint8(cB >> 8),
				A: newA,
			})
		}
	}

	// Rotate tile 30° for diagonal pattern
	rotatedTile := imaging.Rotate(opaqueTile, 30, image.Transparent)

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	tileW = rotatedTile.Bounds().Dx()
	tileH := rotatedTile.Bounds().Dy()
	stepX := tileW + spacing
	stepY := tileH + spacing

	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}

	for y := bounds.Min.Y - tileH; y < bounds.Max.Y+tileH; y += stepY {
		for x := bounds.Min.X - tileW; x < bounds.Max.X+tileW; x += stepX {
			draw.Draw(dst, bounds, rotatedTile, image.Pt(-x, -y), draw.Over)
		}
	}

	return dst
}

// ---------------------------------------------------------------------------
// 3.3 QR Code Overlay (Item 53)
// ---------------------------------------------------------------------------

// applyQRCodeOverlay generates a QR code and overlays it on the image.
func applyQRCodeOverlay(img image.Image, opts *ProcessOptions) image.Image {
	if opts.QRCodeText == "" {
		return img
	}

	qrSize := opts.QRCodeSize
	if qrSize == 0 {
		qrSize = 128
	}
	if qrSize < 64 {
		qrSize = 64
	}

	qr, qErr := qrcode.New(opts.QRCodeText, qrcode.Medium)
	if qErr != nil {
		fmt.Printf("Warning: failed to generate QR code: %v\n", qErr)
		return img
	}
	qrImage := qr.Image(qrSize)

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	padding := 10
	pos := strings.ToLower(opts.QRCodePosition)
	if pos == "" {
		pos = "bottom-right"
	}

	var offsetX, offsetY int
	qrBounds := qrImage.Bounds()
	switch pos {
	case "top-left":
		offsetX = padding
		offsetY = padding
	case "top-right":
		offsetX = bounds.Dx() - qrBounds.Dx() - padding
		offsetY = padding
	case "bottom-left":
		offsetX = padding
		offsetY = bounds.Dy() - qrBounds.Dy() - padding
	default: // bottom-right
		offsetX = bounds.Dx() - qrBounds.Dx() - padding
		offsetY = bounds.Dy() - qrBounds.Dy() - padding
	}

	draw.Draw(dst, bounds, qrImage, image.Pt(-offsetX, -offsetY), draw.Over)
	return dst
}

// ---------------------------------------------------------------------------
// 3.4 Barcode Generator (Item 54)
// ---------------------------------------------------------------------------

// applyBarcodeOverlay generates a barcode and overlays it on the image.
func applyBarcodeOverlay(img image.Image, opts *ProcessOptions) image.Image {
	if opts.BarcodeText == "" {
		return img
	}

	var bc barcode.Barcode
	var bErr error

	switch strings.ToLower(opts.BarcodeType) {
	case "ean13":
		bc, bErr = ean.Encode(opts.BarcodeText)
	default: // code128
		bc, bErr = code128.Encode(opts.BarcodeText)
	}

	if bErr != nil {
		fmt.Printf("Warning: failed to generate barcode: %v\n", bErr)
		return img
	}

	// Scale the barcode to a reasonable size
	bcImage, scaleErr := barcode.Scale(bc, 200, 80)
	if scaleErr != nil {
		fmt.Printf("Warning: failed to scale barcode: %v\n", scaleErr)
		return img
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	// Position at bottom-center with padding
	padding := 10
	bcBounds := bcImage.Bounds()
	offsetX := (bounds.Dx() - bcBounds.Dx()) / 2
	if offsetX < padding {
		offsetX = padding
	}
	offsetY := bounds.Dy() - bcBounds.Dy() - padding

	draw.Draw(dst, bounds, bcImage, image.Pt(-offsetX, -offsetY), draw.Over)
	return dst
}

// ---------------------------------------------------------------------------
// 3.5 Rounded Corners (Item 55)
// ---------------------------------------------------------------------------

// applyRoundedCorners applies rounded corners by making corner pixels transparent.
func applyRoundedCorners(img image.Image, opts *ProcessOptions) image.Image {
	if opts.RoundedCorners <= 0 {
		return img
	}

	radius := opts.RoundedCorners
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Check if pixel is in a corner region
			inCorner := false
			var cx, cy int

			// Top-left corner
			if x < radius && y < radius {
				inCorner = true
				cx = radius
				cy = radius
			}
			// Top-right corner
			if x >= w-radius && y < radius {
				inCorner = true
				cx = w - radius - 1
				cy = radius
			}
			// Bottom-left corner
			if x < radius && y >= h-radius {
				inCorner = true
				cx = radius
				cy = h - radius - 1
			}
			// Bottom-right corner
			if x >= w-radius && y >= h-radius {
				inCorner = true
				cx = w - radius - 1
				cy = h - radius - 1
			}

			if inCorner {
				dx := float64(x - cx)
				dy := float64(y - cy)
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist > float64(radius) {
					// Outside the rounded corner — make transparent
					dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
			}
		}
	}

	return dst
}

// ---------------------------------------------------------------------------
// 3.6 Drop Shadows (Item 56)
// ---------------------------------------------------------------------------

// applyDropShadow adds a drop shadow behind the image.
func applyDropShadow(img image.Image, opts *ProcessOptions) image.Image {
	if opts.DropShadowOffset <= 0 {
		return img
	}

	offset := opts.DropShadowOffset
	blur := opts.DropShadowBlur
	if blur == 0 {
		blur = 5.0
	}

	// Parse shadow color
	sr, sg, sb := parseHexColor(opts.DropShadowColor)
	if opts.DropShadowColor == "" {
		sr, sg, sb = 0, 0, 0 // default black
	}

	// Calculate expanded canvas size
	blurPad := int(blur * 2)
	newW := img.Bounds().Dx() + offset + blurPad
	newH := img.Bounds().Dy() + offset + blurPad

	// Create shadow rectangle (same size as image)
	shadowRect := image.NewNRGBA(img.Bounds())
	for y := shadowRect.Bounds().Min.Y; y < shadowRect.Bounds().Max.Y; y++ {
		for x := shadowRect.Bounds().Min.X; x < shadowRect.Bounds().Max.X; x++ {
			shadowRect.SetNRGBA(x, y, color.NRGBA{R: sr, G: sg, B: sb, A: 128})
		}
	}

	// Apply Gaussian blur to the shadow
	shadowBlurred := gaussianBlur(shadowRect, blur)

	// Create expanded canvas (transparent)
	canvas := image.NewNRGBA(image.Rect(0, 0, newW, newH))

	// Draw the blurred shadow at the offset position
	draw.Draw(canvas, canvas.Bounds(), shadowBlurred, image.Pt(-blurPad/2, -blurPad/2), draw.Over)

	// Draw the original image on top at (0,0)
	draw.Draw(canvas, canvas.Bounds(), img, image.Point{}, draw.Over)

	return canvas
}

// ---------------------------------------------------------------------------
// 3.7 Stroke/Border (Item 57)
// ---------------------------------------------------------------------------

// applyBorder adds a border around the image.
func applyBorder(img image.Image, opts *ProcessOptions) image.Image {
	if opts.BorderWidth <= 0 {
		return img
	}

	bw := opts.BorderWidth
	bounds := img.Bounds()
	newW := bounds.Dx() + 2*bw
	newH := bounds.Dy() + 2*bw

	// Parse border color
	br, bg, bb := parseHexColor(opts.BorderColor)
	if opts.BorderColor == "" {
		br, bg, bb = 0, 0, 0 // default black
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))

	// Fill entire canvas with border color
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			dst.SetNRGBA(x, y, color.NRGBA{R: br, G: bg, B: bb, A: 255})
		}
	}

	// If dashed style, clear alternating segments on the border edges
	if strings.ToLower(opts.BorderStyle) == "dashed" {
		dashOn := 10
		dashOff := 5
		// Top edge
		for x := 0; x < newW; x++ {
			segment := x % (dashOn + dashOff)
			if segment >= dashOn {
				for y := 0; y < bw; y++ {
					dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
			}
		}
		// Bottom edge
		for x := 0; x < newW; x++ {
			segment := x % (dashOn + dashOff)
			if segment >= dashOn {
				for y := newH - bw; y < newH; y++ {
					dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
			}
		}
		// Left edge
		for y := 0; y < newH; y++ {
			segment := y % (dashOn + dashOff)
			if segment >= dashOn {
				for x := 0; x < bw; x++ {
					dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
			}
		}
		// Right edge
		for y := 0; y < newH; y++ {
			segment := y % (dashOn + dashOff)
			if segment >= dashOn {
				for x := newW - bw; x < newW; x++ {
					dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				}
			}
		}
	}

	// Draw the original image centered in the new canvas
	draw.Draw(dst, dst.Bounds(), img, image.Pt(-bw, -bw), draw.Over)

	return dst
}

// ---------------------------------------------------------------------------
// 3.8 Placeholder Generator (Item 58)
// ---------------------------------------------------------------------------

// generatePlaceholder creates a placeholder image with the specified dimensions
// and optional text. This is called from ProcessImage when PlaceholderWidth and
// PlaceholderHeight are set, without needing an input image.
func generatePlaceholder(opts *ProcessOptions) image.Image {
	w := opts.PlaceholderWidth
	h := opts.PlaceholderHeight
	if w < 1 {
		w = 100
	}
	if h < 1 {
		h = 100
	}

	// Parse colors
	bgR, bgG, bgB := parseHexColor(opts.PlaceholderBgColor)
	if opts.PlaceholderBgColor == "" {
		bgR, bgG, bgB = 0xcc, 0xcc, 0xcc
	}
	txtR, txtG, txtB := parseHexColor(opts.PlaceholderTextColor)
	if opts.PlaceholderTextColor == "" {
		txtR, txtG, txtB = 0x66, 0x66, 0x66
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Fill with background color
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetNRGBA(x, y, color.NRGBA{R: bgR, G: bgG, B: bgB, A: 255})
		}
	}

	// Determine text to display
	displayText := opts.PlaceholderText
	if displayText == "" {
		displayText = fmt.Sprintf("%dx%d", w, h)
	}

	// Render text using freetype
	fontPath := os.Getenv("FONT_PATH")
	if fontPath == "" {
		fontPath = getPlatformFontPath()
	}
	if fontPath == "" {
		fmt.Println("Warning: No suitable font found for placeholder text")
		return dst
	}

	fontBytes, fErr := os.ReadFile(filepath.Clean(fontPath)) // #nosec G304
	if fErr != nil {
		fmt.Printf("Warning: failed to load font for placeholder: %v\n", fErr)
		return dst
	}

	f, pErr := freetype.ParseFont(fontBytes)
	if pErr != nil {
		fmt.Printf("Warning: failed to parse font for placeholder: %v\n", pErr)
		return dst
	}

	// Auto-calculate font size to fit the image
	fontSize := float64(w) / 8.0
	if fontSize > float64(h)/4.0 {
		fontSize = float64(h) / 4.0
	}
	if fontSize < 8 {
		fontSize = 8
	}

	fg := image.NewUniform(color.RGBA{txtR, txtG, txtB, 255})
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(fontSize)
	c.SetClip(dst.Bounds())
	c.SetDst(dst)
	c.SetSrc(fg)
	c.SetHinting(font.HintingFull)

	// Center the text
	textWidth := len(displayText) * int(fontSize*0.6)
	xPos := (w - textWidth) / 2
	if xPos < 10 {
		xPos = 10
	}
	yPos := h/2 + int(fontSize*0.35)

	pt := freetype.Pt(xPos, yPos)
	if _, dErr := c.DrawString(displayText, pt); dErr != nil {
		fmt.Printf("Warning: failed to draw placeholder text: %v\n", dErr)
	}

	return dst
}

// ---------------------------------------------------------------------------
// 3.9 Invisible Steganography (Item 59)
// ---------------------------------------------------------------------------

// applySteganography encodes hidden text into the LSB of the red channel.
func applySteganography(img image.Image, opts *ProcessOptions) image.Image {
	if opts.SteganographyText == "" {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	// Prepare data: 4-byte length header + text bytes
	textBytes := []byte(opts.SteganographyText)
	data := make([]byte, 4+len(textBytes))
	binary.BigEndian.PutUint32(data[:4], uint32(len(textBytes)))
	copy(data[4:], textBytes)

	// Check if data fits in the image
	maxBits := bounds.Dx() * bounds.Dy()
	if len(data)*8 > maxBits {
		// Truncate to fit
		maxBytes := maxBits / 8
		if maxBytes > 4 {
			data = data[:maxBytes]
			fmt.Printf("Warning: steganography text truncated to %d bytes\n", maxBytes-4)
		} else {
			fmt.Println("Warning: image too small for steganography")
			return img
		}
	}

	// Encode each bit into the LSB of the red channel
	bitIndex := 0
	for y := bounds.Min.Y; y < bounds.Max.Y && bitIndex < len(data)*8; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && bitIndex < len(data)*8; x++ {
			c := dst.NRGBAAt(x, y)
			byteIndex := bitIndex / 8
			bitPos := uint(7 - (bitIndex % 8))
			bit := (data[byteIndex] >> bitPos) & 1

			// Modify LSB of red channel
			if bit == 1 {
				c.R = c.R | 1
			} else {
				c.R = c.R & 0xFE
			}
			dst.SetNRGBA(x, y, c)
			bitIndex++
		}
	}

	return dst
}

// DecodeSteganography extracts hidden text from the LSB of the red channel.
// This is a utility function for future API use, not called from the pipeline.
func DecodeSteganography(img image.Image) string {
	bounds := img.Bounds()
	totalPixels := bounds.Dx() * bounds.Dy()

	// Need at least 4 bytes (32 bits) for the length header
	if totalPixels < 32 {
		return ""
	}

	// readByte reads 8 bits starting at bitOffset (sequential pixel order)
	readByte := func(bitOffset int) byte {
		var b byte
		for j := 0; j < 8; j++ {
			pixelIdx := bitOffset + j
			x := bounds.Min.X + (pixelIdx % bounds.Dx())
			y := bounds.Min.Y + (pixelIdx / bounds.Dx())
			if x >= bounds.Max.X || y >= bounds.Max.Y {
				continue
			}
			cR, _, _, _ := img.At(x, y).RGBA()
			bit := byte(cR>>8) & 1
			b = b | (bit << uint(7-j))
		}
		return b
	}

	// Read 4-byte length header
	var lengthBytes [4]byte
	for i := 0; i < 4; i++ {
		lengthBytes[i] = readByte(i * 8)
	}
	length := binary.BigEndian.Uint32(lengthBytes[:])

	if length == 0 || int(length) > (totalPixels/8-4) {
		return ""
	}

	// Read text bytes
	result := make([]byte, length)
	for i := 0; i < int(length); i++ {
		result[i] = readByte((4 + i) * 8)
	}

	return string(result)
}

// ---------------------------------------------------------------------------
// 3.10 Signature Stamp (Item 60)
// ---------------------------------------------------------------------------

// applySignature overlays a signature image onto the image.
func applySignature(img image.Image, opts *ProcessOptions, uploadDir string) image.Image {
	if opts.SignaturePath == "" {
		return img
	}

	sig, sErr := imaging.Open(opts.SignaturePath)
	if sErr != nil {
		fmt.Printf("Warning: failed to open signature %s: %v\n", opts.SignaturePath, sErr)
		return img
	}

	// Scale signature
	scale := opts.SignatureScale
	if scale == 0 {
		scale = 1.0
	}
	if scale != 1.0 {
		newW := int(float64(sig.Bounds().Dx()) * scale)
		newH := int(float64(sig.Bounds().Dy()) * scale)
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}
		sig = imaging.Resize(sig, newW, newH, imaging.Lanczos)
	}

	// Apply opacity
	opacity := opts.SignatureOpacity
	if opacity == 0 {
		opacity = 0.8
	}

	sigBounds := sig.Bounds()
	opaqueSig := image.NewNRGBA(sigBounds)
	for y := sigBounds.Min.Y; y < sigBounds.Max.Y; y++ {
		for x := sigBounds.Min.X; x < sigBounds.Max.X; x++ {
			cR, cG, cB, cA := sig.At(x, y).RGBA()
			newA := uint8(float64(cA>>8) * opacity)
			if newA == 0 {
				continue
			}
			opaqueSig.SetNRGBA(x, y, color.NRGBA{
				R: uint8(cR >> 8),
				G: uint8(cG >> 8),
				B: uint8(cB >> 8),
				A: newA,
			})
		}
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	padding := 20
	pos := strings.ToLower(opts.SignaturePosition)
	if pos == "" {
		pos = "bottom-right"
	}

	var offsetX, offsetY int
	sigW := opaqueSig.Bounds().Dx()
	sigH := opaqueSig.Bounds().Dy()
	switch pos {
	case "top-left":
		offsetX = padding
		offsetY = padding
	case "top-right":
		offsetX = bounds.Dx() - sigW - padding
		offsetY = padding
	case "bottom-left":
		offsetX = padding
		offsetY = bounds.Dy() - sigH - padding
	default: // bottom-right
		offsetX = bounds.Dx() - sigW - padding
		offsetY = bounds.Dy() - sigH - padding
	}

	draw.Draw(dst, bounds, opaqueSig, image.Pt(-offsetX, -offsetY), draw.Over)
	return dst
}
