package processor

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/disintegration/imaging"
)

// applyAdjustments applies brightness, contrast, and saturation adjustments.
func applyAdjustments(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Brightness != 0 {
		img = imaging.AdjustBrightness(img, opts.Brightness)
	}
	if opts.Contrast != 0 {
		img = imaging.AdjustContrast(img, opts.Contrast)
	}
	if opts.Saturation != 0 {
		img = imaging.AdjustSaturation(img, opts.Saturation)
	}
	return img
}

// applyVignette adds a radial darkening effect around the edges of the image.
// BUG-10 FIX: Vignette was defined in ProcessOptions but never implemented.
// When VignetteAmount > 0, the new customizable vignette is used instead.
func applyVignette(img image.Image, opts *ProcessOptions) image.Image {
	// Use new customizable vignette when VignetteAmount > 0
	if opts.VignetteAmount > 0 {
		return applyVignetteCustom(img, opts)
	}
	// Legacy vignette behavior for backward compatibility
	if !opts.Vignette {
		return img
	}

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

// ---------------------------------------------------------------------------
// Professional Adjustments
// ---------------------------------------------------------------------------

// applyProfessionalAdjustments calls all professional adjustment functions
// in the correct order. Each sub-function checks if its option is zero/empty
// and returns the image unchanged if so.
func applyProfessionalAdjustments(img image.Image, opts *ProcessOptions) image.Image {
	// 1. Temperature & Tint (white balance first)
	img = applyTemperatureTint(img, opts)
	// 2. Shadow/Highlight Recovery
	img = applyShadowHighlightRecovery(img, opts)
	// 3. HSL
	img = applyHSL(img, opts)
	// 4. Levels
	img = applyLevels(img, opts)
	// 5. Curves
	img = applyCurves(img, opts)
	// 6. Selective Color
	img = applySelectiveColor(img, opts)
	// 7. Chromatic Aberration Fix
	img = applyChromaticAberrationFix(img, opts)
	// 8. Unsharp Mask
	img = applyUnsharpMask(img, opts)
	// 9. Film Grain
	img = applyFilmGrain(img, opts)
	// 10. Vignette Custom is handled inside applyVignette when VignetteAmount > 0
	return img
}

// ---------------------------------------------------------------------------
// 2.1 HSL Sliders (Item 41)
// ---------------------------------------------------------------------------

// rgbToHSL converts RGB values (0-255) to HSL.
// H in [0,360), S in [0,1], L in [0,1].
func rgbToHSL(r, g, b uint8) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	cmax := math.Max(rf, math.Max(gf, bf))
	cmin := math.Min(rf, math.Min(gf, bf))
	delta := cmax - cmin

	l := (cmax + cmin) / 2.0

	if delta == 0 {
		return 0, 0, l
	}

	var s float64
	if l < 0.5 {
		s = delta / (cmax + cmin)
	} else {
		s = delta / (2.0 - cmax - cmin)
	}

	var h float64
	switch {
	case cmax == rf:
		h = math.Mod((gf-bf)/delta, 6.0)
	case cmax == gf:
		h = (bf-rf)/delta + 2.0
	case cmax == bf:
		h = (rf-gf)/delta + 4.0
	}
	h *= 60.0
	if h < 0 {
		h += 360.0
	}

	return h, s, l
}

// hslToRGB converts HSL values to RGB (0-255).
// H in [0,360), S in [0,1], L in [0,1].
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	c := (1.0 - math.Abs(2.0*l-1.0)) * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}

	return clampUint8(int((r1 + m) * 255.0)),
		clampUint8(int((g1 + m) * 255.0)),
		clampUint8(int((b1 + m) * 255.0))
}

func applyHSL(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Hue == 0 && opts.Lightness == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			r, g, b := uint8(cR>>8), uint8(cG>>8), uint8(cB>>8)

			h, s, l := rgbToHSL(r, g, b)

			// Adjust hue (mod 360)
			h = math.Mod(h+opts.Hue+360.0, 360.0)

			// Adjust lightness (clamped 0-100, but L is 0-1)
			l = l + opts.Lightness/100.0
			if l < 0 {
				l = 0
			}
			if l > 1 {
				l = 1
			}

			nr, ng, nb := hslToRGB(h, s, l)
			dst.Set(x, y, color.RGBA{
				R: nr,
				G: ng,
				B: nb,
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.2 Curves Adjustment (Item 42)
// ---------------------------------------------------------------------------

// curvesData represents the JSON structure for curves control points.
type curvesData struct {
	R [][]float64 `json:"r"`
	G [][]float64 `json:"g"`
	B [][]float64 `json:"b"`
}

// cubicSplineLUT builds a 256-entry lookup table from control points using
// cubic spline interpolation.
func cubicSplineLUT(points [][2]float64) [256]uint8 {
	var lut [256]uint8

	if len(points) < 2 {
		// Not enough points; identity mapping
		for i := 0; i < 256; i++ {
			lut[i] = uint8(i)
		}
		return lut
	}

	// Sort by input value
	sort.Slice(points, func(i, j int) bool {
		return points[i][0] < points[j][0]
	})

	n := len(points)

	// Build cubic spline for the output values as a function of input
	// Using natural cubic spline (second derivative = 0 at endpoints)
	// We'll interpolate based on input x -> output y

	// Extract x and y arrays
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i, p := range points {
		xs[i] = p[0]
		ys[i] = p[1]
	}

	// Compute second derivatives (natural spline)
	h := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		h[i] = xs[i+1] - xs[i]
		if h[i] == 0 {
			h[i] = 1e-10
		}
	}

	// Solve tridiagonal system for second derivatives
	d := make([]float64, n)
	mu := make([]float64, n)
	z := make([]float64, n)
	d[0] = 1.0
	mu[0] = 0
	z[0] = 0

	for i := 1; i < n-1; i++ {
		alpha := (3.0/h[i])*ys[i+1] - (3.0/h[i]+3.0/h[i-1])*ys[i] + (3.0/h[i-1])*ys[i-1]
		l := 2.0*(xs[i+1]-xs[i-1]) - h[i-1]*mu[i-1]
		if l == 0 {
			l = 1e-10
		}
		mu[i] = h[i] / l
		z[i] = (alpha - h[i-1]*z[i-1]) / l
		d[i] = 1.0
	}

	d[n-1] = 1.0
	z[n-1] = 0

	c := make([]float64, n)
	b := make([]float64, n-1)
	dd := make([]float64, n-1)

	// Back substitution
	for j := n - 2; j >= 0; j-- {
		c[j] = z[j] - mu[j]*c[j+1]
		b[j] = (ys[j+1]-ys[j])/h[j] - h[j]*(c[j+1]+2.0*c[j])/3.0
		dd[j] = (c[j+1] - c[j]) / (3.0 * h[j])
	}

	// Evaluate spline for each of 256 input values
	for i := 0; i < 256; i++ {
		xVal := float64(i)

		// Clamp to range
		if xVal <= xs[0] {
			lut[i] = clampUint8(int(ys[0]))
			continue
		}
		if xVal >= xs[n-1] {
			lut[i] = clampUint8(int(ys[n-1]))
			continue
		}

		// Find the right interval
		seg := 0
		for j := 0; j < n-1; j++ {
			if xVal >= xs[j] && xVal < xs[j+1] {
				seg = j
				break
			}
		}

		dx := xVal - xs[seg]
		yVal := ys[seg] + b[seg]*dx + c[seg]*dx*dx + dd[seg]*dx*dx*dx
		lut[i] = clampUint8(int(math.Round(yVal)))
	}

	return lut
}

// parseCurvePoints parses a JSON channel array into [][2]float64.
func parseCurvePoints(raw [][]float64) [][2]float64 {
	var points [][2]float64
	for _, p := range raw {
		if len(p) >= 2 {
			points = append(points, [2]float64{p[0], p[1]})
		}
	}
	return points
}

func applyCurves(img image.Image, opts *ProcessOptions) image.Image {
	if opts.CurvesPoints == "" {
		return img
	}

	var cd curvesData
	if err := json.Unmarshal([]byte(opts.CurvesPoints), &cd); err != nil {
		fmt.Printf("Warning: failed to parse curves JSON: %v\n", err)
		return img
	}

	rPoints := parseCurvePoints(cd.R)
	gPoints := parseCurvePoints(cd.G)
	bPoints := parseCurvePoints(cd.B)

	if len(rPoints) < 2 && len(gPoints) < 2 && len(bPoints) < 2 {
		return img
	}

	// Build LUTs; if a channel has <2 points, use identity
	var rLUT, gLUT, bLUT [256]uint8
	if len(rPoints) >= 2 {
		rLUT = cubicSplineLUT(rPoints)
	} else {
		for i := 0; i < 256; i++ {
			rLUT[i] = uint8(i)
		}
	}
	if len(gPoints) >= 2 {
		gLUT = cubicSplineLUT(gPoints)
	} else {
		for i := 0; i < 256; i++ {
			gLUT[i] = uint8(i)
		}
	}
	if len(bPoints) >= 2 {
		bLUT = cubicSplineLUT(bPoints)
	} else {
		for i := 0; i < 256; i++ {
			bLUT[i] = uint8(i)
		}
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: rLUT[uint8(cR>>8)],
				G: gLUT[uint8(cG>>8)],
				B: bLUT[uint8(cB>>8)],
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.3 Levels Histogram (Item 43)
// ---------------------------------------------------------------------------

func applyLevels(img image.Image, opts *ProcessOptions) image.Image {
	black := opts.LevelsBlack
	white := opts.LevelsWhite
	gamma := opts.LevelsGamma

	// Defaults: black=0, white=255, gamma=1.0 → no change
	if black == 0 && white == 255 && (gamma == 0 || gamma == 1.0) {
		return img
	}

	// Normalize zero gamma to 1.0
	if gamma == 0 {
		gamma = 1.0
	}

	// Ensure white > black to avoid division by zero
	if white <= black {
		white = black + 1
	}

	// Build LUT for levels
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		v := float64(i)
		if v < black {
			lut[i] = 0
		} else if v > white {
			lut[i] = 255
		} else {
			normalized := (v - black) / (white - black)
			output := math.Pow(normalized, 1.0/gamma) * 255.0
			lut[i] = clampUint8(int(math.Round(output)))
		}
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: lut[uint8(cR>>8)],
				G: lut[uint8(cG>>8)],
				B: lut[uint8(cB>>8)],
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.4 Selective Color (Item 44)
// ---------------------------------------------------------------------------

// selectiveColorData represents the JSON structure for selective color adjustments.
type selectiveColorData map[string]map[string]float64

// hueRange returns which selective color range a hue falls into.
// Hue ranges: reds (345-15, wrapping), yellows (30-75), greens (75-165),
// cyans (165-195), blues (195-270), magentas (270-345).
func hueRange(h float64) string {
	// Normalize h to [0, 360)
	h = math.Mod(h+360.0, 360.0)

	switch {
	case h >= 345 || h < 15:
		return "reds"
	case h >= 15 && h < 45:
		return "reds" // transition zone, count as reds
	case h >= 45 && h < 75:
		return "yellows"
	case h >= 75 && h < 165:
		return "greens"
	case h >= 165 && h < 195:
		return "cyans"
	case h >= 195 && h < 270:
		return "blues"
	case h >= 270 && h < 345:
		return "magentas"
	default:
		return ""
	}
}

func applySelectiveColor(img image.Image, opts *ProcessOptions) image.Image {
	if opts.SelectiveColor == "" {
		return img
	}

	var sc selectiveColorData
	if err := json.Unmarshal([]byte(opts.SelectiveColor), &sc); err != nil {
		fmt.Printf("Warning: failed to parse selective color JSON: %v\n", err)
		return img
	}

	if len(sc) == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			r, g, b := float64(cR>>8), float64(cG>>8), float64(cB>>8)

			h, _, _ := rgbToHSL(uint8(cR>>8), uint8(cG>>8), uint8(cB>>8))
			rangeName := hueRange(h)

			if adjustments, ok := sc[rangeName]; ok {
				// Cyan adjustment: modify R channel
				if cyanAdj, ok := adjustments["cyan"]; ok {
					r += cyanAdj
				}
				// Magenta adjustment: modify G channel
				if magentaAdj, ok := adjustments["magenta"]; ok {
					g += magentaAdj
				}
				// Yellow adjustment: modify B channel
				if yellowAdj, ok := adjustments["yellow"]; ok {
					b += yellowAdj
				}
				// Black adjustment: modify all channels
				if blackAdj, ok := adjustments["black"]; ok {
					r += blackAdj
					g += blackAdj
					b += blackAdj
				}
			}

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(r)),
				G: clampUint8(int(g)),
				B: clampUint8(int(b)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.5 Chromatic Aberration Fix (Item 45)
// ---------------------------------------------------------------------------

func applyChromaticAberrationFix(img image.Image, opts *ProcessOptions) image.Image {
	if opts.ChromaticAberration == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	cx := float64(bounds.Dx()) / 2.0
	cy := float64(bounds.Dy()) / 2.0
	offset := opts.ChromaticAberration

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			var odx, ody float64
			if dist > 0 {
				odx = dx / dist
				ody = dy / dist
			}

			// Sample R channel shifted inward (toward center)
			rX := float64(x) - odx*offset
			rY := float64(y) - ody*offset
			// Sample B channel shifted outward (away from center)
			bX := float64(x) + odx*offset
			bY := float64(y) + ody*offset

			// Bilinear interpolation for R
			rVal := bilinearSample(img, rX, rY, 'r')
			// G stays at original position
			_, gVal, _, _ := img.At(x, y).RGBA()
			// Bilinear interpolation for B
			bVal := bilinearSample(img, bX, bY, 'b')

			_, _, origB, cA := img.At(x, y).RGBA()
			_ = origB

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(rVal)),
				G: uint8(gVal >> 8),
				B: clampUint8(int(bVal)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// bilinearSample samples a specific channel at sub-pixel coordinates.
func bilinearSample(img image.Image, x, y float64, channel byte) float64 {
	bounds := img.Bounds()

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	fx := x - float64(x0)
	fy := y - float64(y0)

	// Clamp to bounds
	x0c := clampInt(x0, bounds.Min.X, bounds.Max.X-1)
	x1c := clampInt(x1, bounds.Min.X, bounds.Max.X-1)
	y0c := clampInt(y0, bounds.Min.Y, bounds.Max.Y-1)
	y1c := clampInt(y1, bounds.Min.Y, bounds.Max.Y-1)

	v00 := channelAt(img, x0c, y0c, channel)
	v10 := channelAt(img, x1c, y0c, channel)
	v01 := channelAt(img, x0c, y1c, channel)
	v11 := channelAt(img, x1c, y1c, channel)

	return v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy
}

// channelAt returns the float64 value (0-255) of a specific channel at (x,y).
func channelAt(img image.Image, x, y int, ch byte) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	switch ch {
	case 'r':
		return float64(r >> 8)
	case 'g':
		return float64(g >> 8)
	case 'b':
		return float64(b >> 8)
	}
	return 0
}

// clampInt clamps v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// 2.6 Unsharp Mask (Item 46)
// ---------------------------------------------------------------------------

func applyUnsharpMask(img image.Image, opts *ProcessOptions) image.Image {
	if opts.UnsharpAmount == 0 {
		return img
	}

	radius := opts.UnsharpRadius
	if radius < 0.1 {
		radius = 0.1
	}
	amount := opts.UnsharpAmount / 100.0

	blurred := gaussianBlur(img, radius)

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			oR, oG, oB, oA := img.At(x, y).RGBA()
			bR, bG, bB, _ := blurred.At(x, y).RGBA()

			// result = original + amount * (original - blurred)
			rR := float64(oR>>8) + amount*(float64(oR>>8)-float64(bR>>8))
			rG := float64(oG>>8) + amount*(float64(oG>>8)-float64(bG>>8))
			rB := float64(oB>>8) + amount*(float64(oB>>8)-float64(bB>>8))

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(rR)),
				G: clampUint8(int(rG)),
				B: clampUint8(int(rB)),
				A: uint8(oA >> 8),
			})
		}
	}
	return dst
}

// gaussianBlur applies a separable Gaussian blur with the given radius.
func gaussianBlur(img image.Image, radius float64) image.Image {
	sigma := radius / 2.0
	if sigma < 0.5 {
		sigma = 0.5
	}

	// Kernel size = ceil(radius * 3) * 2 + 1
	kernelRadius := int(math.Ceil(sigma * 3.0))
	if kernelRadius < 1 {
		kernelRadius = 1
	}
	kernelSize := kernelRadius*2 + 1

	// Build 1D kernel
	kernel := make([]float64, kernelSize)
	sum := 0.0
	for i := 0; i < kernelSize; i++ {
		x := float64(i - kernelRadius)
		kernel[i] = math.Exp(-(x * x) / (2.0 * sigma * sigma))
		sum += kernel[i]
	}
	// Normalize
	for i := range kernel {
		kernel[i] /= sum
	}

	bounds := img.Bounds()

	// Horizontal pass
	hPass := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var r, g, b, a float64
			for k := 0; k < kernelSize; k++ {
				sx := x + k - kernelRadius
				if sx < bounds.Min.X {
					sx = bounds.Min.X
				}
				if sx >= bounds.Max.X {
					sx = bounds.Max.X - 1
				}
				cR, cG, cB, cA := img.At(sx, y).RGBA()
				w := kernel[k]
				r += float64(cR>>8) * w
				g += float64(cG>>8) * w
				b += float64(cB>>8) * w
				a += float64(cA>>8) * w
			}
			hPass.Set(x, y, color.RGBA{
				R: clampUint8(int(math.Round(r))),
				G: clampUint8(int(math.Round(g))),
				B: clampUint8(int(math.Round(b))),
				A: clampUint8(int(math.Round(a))),
			})
		}
	}

	// Vertical pass
	vPass := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var r, g, b, a float64
			for k := 0; k < kernelSize; k++ {
				sy := y + k - kernelRadius
				if sy < bounds.Min.Y {
					sy = bounds.Min.Y
				}
				if sy >= bounds.Max.Y {
					sy = bounds.Max.Y - 1
				}
				cR, cG, cB, cA := hPass.At(x, sy).RGBA()
				w := kernel[k]
				r += float64(cR>>8) * w
				g += float64(cG>>8) * w
				b += float64(cB>>8) * w
				a += float64(cA>>8) * w
			}
			vPass.Set(x, y, color.RGBA{
				R: clampUint8(int(math.Round(r))),
				G: clampUint8(int(math.Round(g))),
				B: clampUint8(int(math.Round(b))),
				A: clampUint8(int(math.Round(a))),
			})
		}
	}

	return vPass
}

// ---------------------------------------------------------------------------
// 2.7 Film Grain Simulation (Item 47)
// ---------------------------------------------------------------------------

func applyFilmGrain(img image.Image, opts *ProcessOptions) image.Image {
	if opts.GrainAmount == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	// Deterministic seed based on image dimensions for reproducibility
	seed := int64(bounds.Dx()*1000 + bounds.Dy())
	rng := rand.New(rand.NewSource(seed))

	grainScale := (opts.GrainAmount / 100.0) * 50.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()

			noiseR := (rng.Float64() - 0.5) * 2.0 * grainScale
			noiseG := (rng.Float64() - 0.5) * 2.0 * grainScale
			noiseB := (rng.Float64() - 0.5) * 2.0 * grainScale

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(float64(cR>>8) + noiseR)),
				G: clampUint8(int(float64(cG>>8) + noiseG)),
				B: clampUint8(int(float64(cB>>8) + noiseB)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.8 Temperature & Tint (Item 48)
// ---------------------------------------------------------------------------

func applyTemperatureTint(img image.Image, opts *ProcessOptions) image.Image {
	if opts.Temperature == 0 && opts.Tint == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			r := float64(cR>>8) + opts.Temperature*0.5
			g := float64(cG>>8) + opts.Tint*0.3
			b := float64(cB>>8) - opts.Temperature*0.5

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(r)),
				G: clampUint8(int(g)),
				B: clampUint8(int(b)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.9 Shadow/Highlight Recovery (Item 49)
// ---------------------------------------------------------------------------

func applyShadowHighlightRecovery(img image.Image, opts *ProcessOptions) image.Image {
	if opts.ShadowRecovery == 0 && opts.HighlightRecovery == 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cR, cG, cB, cA := img.At(x, y).RGBA()
			r, g, b := float64(cR>>8), float64(cG>>8), float64(cB>>8)

			// Calculate luminance
			lum := 0.299*r + 0.587*g + 0.114*b

			var adj float64

			// Shadow recovery: brighten dark pixels
			if lum < 128 && opts.ShadowRecovery > 0 {
				adj = (opts.ShadowRecovery / 100.0) * (1.0 - lum/128.0) * 50.0
				r += adj
				g += adj
				b += adj
			}

			// Highlight recovery: darken bright pixels
			if lum > 128 && opts.HighlightRecovery > 0 {
				adj = (opts.HighlightRecovery / 100.0) * ((lum - 128.0) / 128.0) * 50.0
				r -= adj
				g -= adj
				b -= adj
			}

			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(r)),
				G: clampUint8(int(g)),
				B: clampUint8(int(b)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// 2.10 Vignette Customization (Item 50)
// ---------------------------------------------------------------------------

// smoothstep performs Hermite interpolation between edge0 and edge1.
func smoothstep(edge0, edge1, x float64) float64 {
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3.0 - 2.0*t)
}

func applyVignetteCustom(img image.Image, opts *ProcessOptions) image.Image {
	if opts.VignetteAmount <= 0 {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Src)

	cx := float64(bounds.Dx()) / 2.0
	cy := float64(bounds.Dy()) / 2.0

	amount := opts.VignetteAmount / 100.0
	feather := opts.VignetteFeather / 100.0
	roundness := opts.VignetteRoundness / 100.0
	midpoint := opts.VignetteMidpoint / 100.0

	// Default feather to 0.5 if not set
	if feather == 0 && opts.VignetteFeather == 0 {
		feather = 0.5
	}
	// Default midpoint to 0.5 if not set
	if midpoint == 0 && opts.VignetteMidpoint == 0 {
		midpoint = 0.5
	}

	// Calculate max distances for elliptical vs circular
	maxDistEllipse := math.Sqrt(cx*cx + cy*cy)
	maxDistCircle := math.Min(cx, cy)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy

			// Normalize distance based on roundness
			// roundness=0: elliptical (matches aspect ratio)
			// roundness=1: circular
			ellipDist := math.Sqrt((dx/cx)*(dx/cx)+(dy/cy)*(dy/cy)) * maxDistEllipse
			circDist := math.Sqrt(dx*dx+dy*dy) / maxDistCircle
			if maxDistCircle == 0 {
				circDist = 0
			}

			// Blend between elliptical and circular based on roundness
			adjustedDist := ellipDist*(1.0-roundness) + circDist*roundness

			// Normalize to 0-1 range
			normalizedDist := adjustedDist / maxDistEllipse
			if normalizedDist > 1.0 {
				normalizedDist = 1.0
			}

			// Apply midpoint: shift where darkening starts
			// midpoint=0: darkening from center, midpoint=1: from edges only
			shiftedDist := (normalizedDist - midpoint) / (1.0 - midpoint + 0.001)

			// Apply feather (smoothstep transition)
			// feather=0: hard edge, feather=1: very gradual
			edgeStart := 1.0 - feather
			factor := 1.0 - amount*smoothstep(edgeStart, 1.0, shiftedDist)
			if factor < 0 {
				factor = 0
			}

			cR, cG, cB, cA := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: clampUint8(int(float64(cR>>8) * factor)),
				G: clampUint8(int(float64(cG>>8) * factor)),
				B: clampUint8(int(float64(cB>>8) * factor)),
				A: uint8(cA >> 8),
			})
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// Unused import guard — time is used by film grain seed in future extensions
// ---------------------------------------------------------------------------

var _ = time.Second // ensure time import is used
