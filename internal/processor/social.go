package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"golang.org/x/image/font"
)

// ---------------------------------------------------------------------------
// 4.1 Instagram Carousel Slicer (Item 81)
// ---------------------------------------------------------------------------

// applyCarouselSlice splits a wide image into multiple carousel slides.
// Returns a slice of slide images, or nil if carousel slicing is disabled.
func applyCarouselSlice(img image.Image, opts *ProcessOptions) []image.Image {
	if !opts.CarouselSlice {
		return nil
	}

	slideW := opts.CarouselSliceWidth
	if slideW <= 0 {
		slideW = 1080
	}
	slideH := opts.CarouselSliceHeight
	if slideH <= 0 {
		slideH = 1350
	}

	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	if srcW <= 0 || srcH <= 0 {
		return nil
	}

	// Calculate number of slides needed
	numSlides := int(math.Ceil(float64(srcW) / float64(slideW)))
	if numSlides < 1 {
		numSlides = 1
	}

	var slides []image.Image

	for i := 0; i < numSlides; i++ {
		// Create a new white canvas for each slide
		slide := image.NewNRGBA(image.Rect(0, 0, slideW, slideH))
		// Fill with white
		for y := 0; y < slideH; y++ {
			for x := 0; x < slideW; x++ {
				slide.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}

		// Calculate the source strip bounds
		srcXStart := srcBounds.Min.X + i*slideW
		srcXEnd := srcXStart + slideW
		if srcXEnd > srcBounds.Max.X {
			srcXEnd = srcBounds.Max.X
		}
		stripW := srcXEnd - srcXStart

		if stripW <= 0 {
			continue
		}

		// Determine vertical positioning: center-crop if taller, center-pad if shorter
		srcStripH := srcH
		var srcYStart int
		var dstYStart int

		if srcStripH > slideH {
			// Source is taller than slide — center-crop vertically
			srcYStart = srcBounds.Min.Y + (srcStripH-slideH)/2
			srcStripH = slideH
			dstYStart = 0
		} else {
			// Source is shorter — center it vertically with white padding
			srcYStart = srcBounds.Min.Y
			dstYStart = (slideH - srcStripH) / 2
		}

		// Copy the strip from source to the slide
		for y := 0; y < srcStripH; y++ {
			for x := 0; x < stripW; x++ {
				srcX := srcXStart + x
				srcY := srcYStart + y
				dstX := x
				dstY := dstYStart + y
				if dstX < slideW && dstY < slideH {
					r, g, b, a := img.At(srcX, srcY).RGBA()
					slide.SetNRGBA(dstX, dstY, color.NRGBA{
						R: uint8(r >> 8),
						G: uint8(g >> 8),
						B: uint8(b >> 8),
						A: uint8(a >> 8),
					})
				}
			}
		}

		slides = append(slides, slide)
	}

	return slides
}

// ---------------------------------------------------------------------------
// 4.2 YouTube Thumbnail Safe-Zone (Item 82)
// ---------------------------------------------------------------------------

// applySafeZoneOverlay draws semi-transparent safe zone guides on the image.
func applySafeZoneOverlay(img image.Image, opts *ProcessOptions) image.Image {
	if !opts.SafeZoneOverlay {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, img, image.Point{}, draw.Src)

	platform := strings.ToLower(opts.SafeZonePlatform)

	switch platform {
	case "youtube":
		// Timestamp area: bottom-right 25% of image, ~15% height
		tsX := int(float64(w) * 0.75)
		tsY := int(float64(h) * 0.85)
		tsW := w - tsX
		tsH := h - tsY
		drawSemiTransparentRect(dst, tsX, tsY, tsW, tsH, color.NRGBA{R: 255, G: 0, B: 0, A: 80})
		drawRectBorder(dst, tsX, tsY, tsW, tsH, color.NRGBA{R: 255, G: 0, B: 0, A: 120})

		// Channel info area: bottom-right 30% width, bottom 20% height
		ciX := int(float64(w) * 0.70)
		ciY := int(float64(h) * 0.80)
		ciW := w - ciX
		ciH := h - ciY
		drawSemiTransparentRect(dst, ciX, ciY, ciW, ciH, color.NRGBA{R: 255, G: 165, B: 0, A: 60})
		drawRectBorder(dst, ciX, ciY, ciW, ciH, color.NRGBA{R: 255, G: 165, B: 0, A: 100})

		// Safe zone: center-left 60%, top 70%
		szX := 0
		szY := 0
		szW := int(float64(w) * 0.60)
		szH := int(float64(h) * 0.70)
		drawSemiTransparentRect(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 40})
		drawRectBorder(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 120})

		drawZoneLabel(dst, "Safe Zone", szX+10, szY+20, color.NRGBA{R: 0, G: 255, B: 0, A: 200})
		drawZoneLabel(dst, "Timestamp", tsX+5, tsY+15, color.NRGBA{R: 255, G: 0, B: 0, A: 200})
		drawZoneLabel(dst, "Channel Info", ciX+5, ciY+15, color.NRGBA{R: 255, G: 165, B: 0, A: 200})

	case "twitter":
		// Profile picture overlap: circular area bottom-left
		profileR := int(math.Min(float64(w)*0.08, 50))
		profileCX := int(float64(w) * 0.08)
		profileCY := h - profileR - 10
		drawSemiTransparentCircle(dst, profileCX, profileCY, profileR, color.NRGBA{R: 255, G: 0, B: 0, A: 80})
		drawZoneLabel(dst, "Profile Pic", profileCX+profileR+5, profileCY, color.NRGBA{R: 255, G: 0, B: 0, A: 200})

		// Safe text zone in center
		szX := int(float64(w) * 0.10)
		szY := int(float64(h) * 0.10)
		szW := int(float64(w) * 0.80)
		szH := int(float64(h) * 0.70)
		drawSemiTransparentRect(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 40})
		drawRectBorder(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 120})
		drawZoneLabel(dst, "Safe Zone", szX+10, szY+20, color.NRGBA{R: 0, G: 255, B: 0, A: 200})

	case "linkedin":
		// Profile picture area: bottom-left corner
		ppSize := int(math.Min(float64(w)*0.12, 80))
		ppX := 0
		ppY := h - ppSize
		drawSemiTransparentRect(dst, ppX, ppY, ppSize, ppSize, color.NRGBA{R: 255, G: 0, B: 0, A: 80})
		drawRectBorder(dst, ppX, ppY, ppSize, ppSize, color.NRGBA{R: 255, G: 0, B: 0, A: 120})
		drawZoneLabel(dst, "Profile Pic", ppX+ppSize+5, ppY+ppSize/2, color.NRGBA{R: 255, G: 0, B: 0, A: 200})

		// Safe zone for banner text
		szX := int(float64(w) * 0.05)
		szY := int(float64(h) * 0.10)
		szW := int(float64(w) * 0.70)
		szH := int(float64(h) * 0.70)
		drawSemiTransparentRect(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 40})
		drawRectBorder(dst, szX, szY, szW, szH, color.NRGBA{R: 0, G: 255, B: 0, A: 120})
		drawZoneLabel(dst, "Safe Zone", szX+10, szY+20, color.NRGBA{R: 0, G: 255, B: 0, A: 200})
	}

	return dst
}

// drawSemiTransparentRect fills a rectangle with a semi-transparent color.
func drawSemiTransparentRect(dst *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && px < dst.Bounds().Dx() && py >= 0 && py < dst.Bounds().Dy() {
				// Alpha blend
				bg := dst.NRGBAAt(px, py)
				alphaF := float64(c.A) / 255.0
				r := uint8(float64(c.R)*alphaF + float64(bg.R)*(1-alphaF))
				g := uint8(float64(c.G)*alphaF + float64(bg.G)*(1-alphaF))
				b := uint8(float64(c.B)*alphaF + float64(bg.B)*(1-alphaF))
				dst.SetNRGBA(px, py, color.NRGBA{R: r, G: g, B: b, A: bg.A})
			}
		}
	}
}

// drawRectBorder draws a 2px border around a rectangle.
func drawRectBorder(dst *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for i := 0; i < w; i++ {
		setNRGBASafe(dst, x+i, y, c)
		setNRGBASafe(dst, x+i, y+1, c)
		setNRGBASafe(dst, x+i, y+h-1, c)
		setNRGBASafe(dst, x+i, y+h-2, c)
	}
	for i := 0; i < h; i++ {
		setNRGBASafe(dst, x, y+i, c)
		setNRGBASafe(dst, x+1, y+i, c)
		setNRGBASafe(dst, x+w-1, y+i, c)
		setNRGBASafe(dst, x+w-2, y+i, c)
	}
}

// drawSemiTransparentCircle fills a circle with a semi-transparent color.
func drawSemiTransparentCircle(dst *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			if dx*dx+dy*dy <= float64(r*r) {
				if x >= 0 && x < dst.Bounds().Dx() && y >= 0 && y < dst.Bounds().Dy() {
					bg := dst.NRGBAAt(x, y)
					alphaF := float64(c.A) / 255.0
					rCh := uint8(float64(c.R)*alphaF + float64(bg.R)*(1-alphaF))
					gCh := uint8(float64(c.G)*alphaF + float64(bg.G)*(1-alphaF))
					bCh := uint8(float64(c.B)*alphaF + float64(bg.B)*(1-alphaF))
					dst.SetNRGBA(x, y, color.NRGBA{R: rCh, G: gCh, B: bCh, A: bg.A})
				}
			}
		}
	}
}

// drawZoneLabel renders a small text label on the image using freetype.
func drawZoneLabel(dst *image.NRGBA, text string, x, y int, c color.NRGBA) {
	fontPath := os.Getenv("FONT_PATH")
	if fontPath == "" {
		fontPath = getPlatformFontPath()
	}
	if fontPath == "" {
		return
	}

	fontBytes, fErr := os.ReadFile(filepath.Clean(fontPath))
	if fErr != nil {
		return
	}

	f, pErr := freetype.ParseFont(fontBytes)
	if pErr != nil {
		return
	}

	fg := image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(f)
	ctx.SetFontSize(14)
	ctx.SetClip(dst.Bounds())
	ctx.SetDst(dst)
	ctx.SetSrc(fg)
	ctx.SetHinting(font.HintingFull)

	pt := freetype.Pt(x, y+int(ctx.PointToFixed(14)>>6))
	if _, dErr := ctx.DrawString(text, pt); dErr != nil {
		fmt.Printf("Warning: failed to draw zone label: %v\n", dErr)
	}
}

// setNRGBASafe sets a pixel if within bounds.
func setNRGBASafe(dst *image.NRGBA, x, y int, c color.NRGBA) {
	if x >= 0 && x < dst.Bounds().Dx() && y >= 0 && y < dst.Bounds().Dy() {
		dst.SetNRGBA(x, y, c)
	}
}

// ---------------------------------------------------------------------------
// 4.3 Discord Emoji Optimizer (Item 83)
// ---------------------------------------------------------------------------

// applyFileSizeOptimization reduces file size by lowering quality and/or
// dimensions until the encoded image fits within MaxFileSizeKB.
func applyFileSizeOptimization(img image.Image, opts *ProcessOptions, format string) image.Image {
	if opts.MaxFileSizeKB <= 0 {
		return img
	}

	maxBytes := opts.MaxFileSizeKB * 1024
	quality := opts.Quality
	if quality <= 0 {
		quality = 85
	}

	currentImg := img
	dimScale := 1.0
	maxIterations := 20

	for i := 0; i < maxIterations; i++ {
		var buf bytes.Buffer

		// Encode to buffer
		switch strings.ToLower(format) {
		case "png":
			if err := imaging.Encode(&buf, currentImg, imaging.PNG); err != nil {
				break
			}
		case "webp":
			if err := imaging.Encode(&buf, currentImg, imaging.PNG); err != nil {
				break
			}
		default: // jpeg
			if err := imaging.Encode(&buf, currentImg, imaging.JPEG, imaging.JPEGQuality(quality)); err != nil {
				break
			}
		}

		if buf.Len() <= maxBytes {
			return currentImg
		}

		// Reduce quality first
		if quality > 10 {
			quality -= 10
			continue
		}

		// Quality at minimum — reduce dimensions by 10%
		dimScale *= 0.9
		bounds := currentImg.Bounds()
		newW := int(float64(bounds.Dx()) * dimScale)
		newH := int(float64(bounds.Dy()) * dimScale)
		if newW < 8 || newH < 8 {
			// Minimum size reached
			return currentImg
		}
		currentImg = imaging.Resize(currentImg, newW, newH, imaging.Lanczos)
		quality = opts.Quality
		if quality <= 0 {
			quality = 85
		}
	}

	return currentImg
}

// ---------------------------------------------------------------------------
// 4.4 App Store Screenshot Mockups (Item 84)
// ---------------------------------------------------------------------------

// applyDeviceMockup overlays the image onto a device frame PNG.
func applyDeviceMockup(img image.Image, opts *ProcessOptions) image.Image {
	if opts.DeviceFramePath == "" {
		return img
	}

	frame, fErr := imaging.Open(opts.DeviceFramePath)
	if fErr != nil {
		fmt.Printf("Warning: failed to open device frame %s: %v\n", opts.DeviceFramePath, fErr)
		return img
	}

	frameBounds := frame.Bounds()
	frameW := frameBounds.Dx()
	frameH := frameBounds.Dy()

	// Resize source image to fit the device screen area (assume 80% of frame)
	screenW := int(float64(frameW) * 0.80)
	screenH := int(float64(frameH) * 0.80)
	if screenW < 1 {
		screenW = 1
	}
	if screenH < 1 {
		screenH = 1
	}

	resizedSrc := imaging.Fill(img, screenW, screenH, imaging.Center, imaging.Lanczos)

	// Create canvas the size of the device frame
	canvas := image.NewNRGBA(image.Rect(0, 0, frameW, frameH))

	// Draw the resized source image centered (offset for screen area)
	offsetX := (frameW - screenW) / 2
	offsetY := (frameH - screenH) / 2
	draw.Draw(canvas, canvas.Bounds(), resizedSrc, image.Pt(-offsetX, -offsetY), draw.Over)

	// Draw the device frame on top
	draw.Draw(canvas, canvas.Bounds(), frame, image.Point{}, draw.Over)

	return canvas
}

// ---------------------------------------------------------------------------
// 4.7 Slack Emoji Auto-Resize (Item 87)
// ---------------------------------------------------------------------------

// applySlackEmojiResize resizes the image to exactly 128x128 pixels.
func applySlackEmojiResize(img image.Image, opts *ProcessOptions) image.Image {
	// Only resize to 128x128 if the social preset is "slack-emoji" or
	// the image is already small enough to be an emoji.
	// We check if the target dimensions are 128x128 (set by preset).
	if opts.Width == 128 && opts.Height == 128 {
		return imaging.Resize(img, 128, 128, imaging.Lanczos)
	}
	return img
}

// ---------------------------------------------------------------------------
// 4.8 Pinterest Long-Pin Stitcher (Item 88)
// ---------------------------------------------------------------------------

// applyImageStitch stitches multiple images vertically or horizontally.
// For single-image uploads, this is a no-op.
func applyImageStitch(img image.Image, opts *ProcessOptions, additionalImages []image.Image) image.Image {
	if !opts.StitchImages {
		return img
	}

	if len(additionalImages) == 0 {
		// Single image — no stitching needed
		return img
	}

	allImages := append([]image.Image{img}, additionalImages...)

	direction := strings.ToLower(opts.StitchDirection)
	if direction == "" {
		direction = "vertical"
	}

	switch direction {
	case "vertical":
		return stitchVertical(allImages)
	case "horizontal":
		return stitchHorizontal(allImages)
	default:
		return stitchVertical(allImages)
	}
}

// stitchVertical stacks images vertically with a max width of 1000px (Pinterest).
func stitchVertical(images []image.Image) image.Image {
	if len(images) == 0 {
		return nil
	}

	// Find max width and calculate total height
	maxW := 0
	totalH := 0
	for _, img := range images {
		b := img.Bounds()
		if b.Dx() > maxW {
			maxW = b.Dx()
		}
		totalH += b.Dy()
	}

	// Pinterest max width
	if maxW > 1000 {
		maxW = 1000
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, maxW, totalH))
	// Fill with white
	for y := 0; y < totalH; y++ {
		for x := 0; x < maxW; x++ {
			canvas.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	currentY := 0
	for _, img := range images {
		b := img.Bounds()
		// Center horizontally
		offsetX := (maxW - b.Dx()) / 2
		draw.Draw(canvas, canvas.Bounds(), img, image.Pt(-offsetX, -currentY), draw.Over)
		currentY += b.Dy()
	}

	return canvas
}

// stitchHorizontal places images side by side.
func stitchHorizontal(images []image.Image) image.Image {
	if len(images) == 0 {
		return nil
	}

	// Find max height and calculate total width
	maxH := 0
	totalW := 0
	for _, img := range images {
		b := img.Bounds()
		if b.Dy() > maxH {
			maxH = b.Dy()
		}
		totalW += b.Dx()
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, totalW, maxH))
	// Fill with white
	for y := 0; y < maxH; y++ {
		for x := 0; x < totalW; x++ {
			canvas.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	currentX := 0
	for _, img := range images {
		b := img.Bounds()
		// Center vertically
		offsetY := (maxH - b.Dy()) / 2
		draw.Draw(canvas, canvas.Bounds(), img, image.Pt(-currentX, -offsetY), draw.Over)
		currentX += b.Dx()
	}

	return canvas
}

// ---------------------------------------------------------------------------
// 4.9 Favicon Generator (Item 89)
// ---------------------------------------------------------------------------

// generateFavicons creates favicon images at multiple sizes.
// Returns a map of size→image, or nil if favicon generation is disabled.
func generateFavicons(img image.Image, opts *ProcessOptions) map[int]image.Image {
	if !opts.FaviconGenerate {
		return nil
	}

	sizesStr := opts.FaviconSizes
	if sizesStr == "" {
		sizesStr = "16,32,48,64,128,180,192,256,384,512"
	}

	sizeStrs := strings.Split(sizesStr, ",")
	result := make(map[int]image.Image)

	for _, s := range sizeStrs {
		s = strings.TrimSpace(s)
		size, err := strconv.Atoi(s)
		if err != nil || size <= 0 {
			continue
		}
		if size < 1 {
			size = 1
		}
		resized := imaging.Resize(img, size, size, imaging.Lanczos)
		result[size] = resized
	}

	return result
}

// generateFaviconManifest creates a Web App Manifest JSON file for PWA use.
func generateFaviconManifest(opts *ProcessOptions, manifestPath string) error {
	sizesStr := opts.FaviconSizes
	if sizesStr == "" {
		sizesStr = "16,32,48,64,128,180,192,256,384,512"
	}

	type ManifestIcon struct {
		Src   string `json:"src"`
		Sizes string `json:"sizes"`
		Type  string `json:"type"`
	}

	type Manifest struct {
		Name            string         `json:"name"`
		ShortName       string         `json:"short_name"`
		Icons           []ManifestIcon `json:"icons"`
		StartURL        string         `json:"start_url"`
		Display         string         `json:"display"`
		BackgroundColor string         `json:"background_color"`
	}

	var icons []ManifestIcon
	for _, s := range strings.Split(sizesStr, ",") {
		s = strings.TrimSpace(s)
		size, err := strconv.Atoi(s)
		if err != nil || size <= 0 {
			continue
		}
		// Only include common PWA sizes in manifest
		if size == 192 || size == 512 {
			icons = append(icons, ManifestIcon{
				Src:   fmt.Sprintf("favicon-%dx%d.png", size, size),
				Sizes: fmt.Sprintf("%dx%d", size, size),
				Type:  "image/png",
			})
		}
	}

	manifest := Manifest{
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

	return os.WriteFile(filepath.Clean(manifestPath), data, 0600)
}
