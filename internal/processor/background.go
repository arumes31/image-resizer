package processor

import (
	"container/list"
	"image"
	"image/color"
	"math"
)

// applyBackgroundRemoval is the main entry point for background removal.
// It dispatches to the appropriate method based on BgRemovalMethod.
func applyBackgroundRemoval(img image.Image, opts *ProcessOptions) image.Image {
	if !opts.RemoveBackground {
		return img
	}

	method := opts.BgRemovalMethod
	if method == "" {
		method = "transparent"
	}

	// Convert to NRGBA first so we have a consistent alpha-capable model
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}

	switch method {
	case "transparent":
		nrgba = removeTransparent(nrgba, opts)
	case "flood-fill":
		nrgba = removeFloodFill(nrgba, opts)
	case "color-match":
		nrgba = removeColorMatch(nrgba, opts)
	}

	// Apply edge smoothing if requested
	if opts.BgRemovalEdgeSmooth > 0 {
		nrgba = smoothEdges(nrgba, opts.BgRemovalEdgeSmooth)
	}

	return nrgba
}

// removeTransparent removes solid color backgrounds by sampling corners.
func removeTransparent(img *image.NRGBA, opts *ProcessOptions) *image.NRGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w == 0 || h == 0 {
		return img
	}

	// Sample 5x5 areas from each corner
	sampleSize := 5
	topLeft := sampleCornerColor(img, bounds.Min.X, bounds.Min.Y, sampleSize)
	topRight := sampleCornerColor(img, bounds.Max.X-sampleSize, bounds.Min.Y, sampleSize)
	bottomLeft := sampleCornerColor(img, bounds.Min.X, bounds.Max.Y-sampleSize, sampleSize)
	bottomRight := sampleCornerColor(img, bounds.Max.X-sampleSize, bounds.Max.Y-sampleSize, sampleSize)

	// Find the most common corner color (likely the background)
	bgColor := findMostCommonCornerColor(topLeft, topRight, bottomLeft, bottomRight)

	// Map tolerance (0-100) to RGB distance (0-150)
	tolerance := float64(opts.BgRemovalTolerance) * 1.5

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			dist := colorDistance(c, bgColor)
			if dist < tolerance {
				img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0})
			}
		}
	}

	return img
}

// removeFloodFill removes background by flood-filling from edges.
func removeFloodFill(img *image.NRGBA, opts *ProcessOptions) *image.NRGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w == 0 || h == 0 {
		return img
	}

	// Map tolerance (0-100) to RGB distance (0-150)
	tolerance := float64(opts.BgRemovalTolerance) * 1.5

	// visited tracks pixels already processed by BFS
	visited := make(map[[2]int]bool)
	// bgPixels tracks pixels identified as background
	bgPixels := make(map[[2]int]bool)

	// BFS queue using container/list for efficiency
	queue := list.New()

	// Helper to add edge pixel to queue if not visited
	enqueueEdge := func(x, y int) {
		key := [2]int{x, y}
		if !visited[key] {
			visited[key] = true
			queue.PushBack(key)
		}
	}

	// Seed from all edge pixels
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		enqueueEdge(x, bounds.Min.Y)   // top row
		enqueueEdge(x, bounds.Max.Y-1) // bottom row
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		enqueueEdge(bounds.Min.X, y)   // left column
		enqueueEdge(bounds.Max.X-1, y) // right column
	}

	// Get the color of the first edge pixel as reference
	// (we'll compare each pixel to its own starting color, not a single reference)
	// Store the starting color for each pixel we visit from the edge

	// For each edge pixel, store its original color as the reference
	// We use a map to store the reference color for each seed pixel
	seedColors := make(map[[2]int]color.NRGBA)

	// Re-seed: record colors of all edge pixels
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		key1 := [2]int{x, bounds.Min.Y}
		seedColors[key1] = img.NRGBAAt(x, bounds.Min.Y)
		key2 := [2]int{x, bounds.Max.Y - 1}
		seedColors[key2] = img.NRGBAAt(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		key1 := [2]int{bounds.Min.X, y}
		seedColors[key1] = img.NRGBAAt(bounds.Min.X, y)
		key2 := [2]int{bounds.Max.X - 1, y}
		seedColors[key2] = img.NRGBAAt(bounds.Max.X-1, y)
	}

	// Process BFS
	for queue.Len() > 0 {
		front := queue.Front()
		queue.Remove(front)
		coord := front.Value.([2]int)
		cx, cy := coord[0], coord[1]

		pixelColor := img.NRGBAAt(cx, cy)
		refColor, hasRef := seedColors[coord]

		// If this pixel's color is within tolerance of its seed color, it's background
		if hasRef {
			dist := colorDistance(pixelColor, refColor)
			if dist < tolerance {
				bgPixels[coord] = true

				// Add 4-connected neighbors
				neighbors := [4][2]int{
					{cx - 1, cy},
					{cx + 1, cy},
					{cx, cy - 1},
					{cx, cy + 1},
				}
				for _, n := range neighbors {
					nx, ny := n[0], n[1]
					if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
						nkey := [2]int{nx, ny}
						if !visited[nkey] {
							visited[nkey] = true
							// Propagate the seed color to neighbors so they compare
							// against the edge color they originated from
							seedColors[nkey] = refColor
							queue.PushBack(nkey)
						}
					}
				}
			}
		}
	}

	// Set all background pixels to transparent
	for key := range bgPixels {
		x, y := key[0], key[1]
		c := img.NRGBAAt(x, y)
		img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0})
	}

	return img
}

// removeColorMatch removes all pixels matching a specific color.
func removeColorMatch(img *image.NRGBA, opts *ProcessOptions) *image.NRGBA {
	bounds := img.Bounds()

	if bounds.Empty() {
		return img
	}

	// Parse the target hex color using the existing utility
	r, g, b := parseHexColor(opts.BgRemovalColor)
	targetColor := color.NRGBA{R: r, G: g, B: b, A: 255}

	// Map tolerance (0-100) to RGB distance (0-150)
	tolerance := float64(opts.BgRemovalTolerance) * 1.5

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			dist := colorDistance(c, targetColor)
			if dist < tolerance {
				img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0})
			}
		}
	}

	return img
}

// smoothEdges applies alpha feathering to transparent pixels that have
// non-transparent neighbors within the given radius.
// This creates a smooth transition between transparent and opaque areas.
func smoothEdges(img *image.NRGBA, radius int) *image.NRGBA {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w == 0 || h == 0 || radius <= 0 {
		return img
	}

	result := image.NewNRGBA(bounds)
	// Copy original to result
	copy(result.Pix, img.Pix)

	// For each pixel, if it's on the border between transparent and opaque,
	// apply a box blur on the alpha channel only
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.NRGBAAt(x, y)

			// Only process pixels that are partially or fully transparent
			// and have at least one non-transparent neighbor within radius
			if c.A == 255 {
				continue
			}

			// Check if this pixel is near the border (has opaque neighbors)
			hasOpaqueNeighbor := false
			for dy := -radius; dy <= radius && !hasOpaqueNeighbor; dy++ {
				for dx := -radius; dx <= radius && !hasOpaqueNeighbor; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
						if img.NRGBAAt(nx, ny).A > 0 {
							hasOpaqueNeighbor = true
						}
					}
				}
			}

			if !hasOpaqueNeighbor {
				continue
			}

			// Box blur on alpha channel within radius
			var alphaSum uint32
			var count uint32

			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
						alphaSum += uint32(img.NRGBAAt(nx, ny).A)
						count++
					}
				}
			}

			if count > 0 {
				newAlpha := uint8(alphaSum / count)
				result.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: newAlpha})
			}
		}
	}

	return result
}

// colorDistance calculates the Euclidean distance between two colors in RGB space.
// Alpha is ignored for the distance calculation.
func colorDistance(c1, c2 color.Color) float64 {
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()

	// RGBA() returns premultiplied values in 0-65535 range, convert to 0-255
	r1f := float64(r1>>8) / 255.0 * 255.0
	g1f := float64(g1>>8) / 255.0 * 255.0
	b1f := float64(b1>>8) / 255.0 * 255.0
	r2f := float64(r2>>8) / 255.0 * 255.0
	g2f := float64(g2>>8) / 255.0 * 255.0
	b2f := float64(b2>>8) / 255.0 * 255.0

	dr := r1f - r2f
	dg := g1f - g2f
	db := b1f - b2f

	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// sampleCornerColor averages the color of a size×size area starting at (x,y).
func sampleCornerColor(img image.Image, x, y, size int) color.Color {
	var rSum, gSum, bSum uint32
	var count uint32

	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			px, py := x+dx, y+dy
			if px >= img.Bounds().Min.X && px < img.Bounds().Max.X &&
				py >= img.Bounds().Min.Y && py < img.Bounds().Max.Y {
				r, g, b, _ := img.At(px, py).RGBA()
				rSum += r >> 8
				gSum += g >> 8
				bSum += b >> 8
				count++
			}
		}
	}

	if count == 0 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}

	return color.NRGBA{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
		A: 255,
	}
}

// findMostCommonCornerColor determines the most common color among the four corners.
// It groups similar colors together (within a threshold) and returns the most frequent group.
func findMostCommonCornerColor(c1, c2, c3, c4 color.Color) color.Color {
	corners := []color.Color{c1, c2, c3, c4}

	// Group similar colors together
	type group struct {
		representative color.Color
		count          int
	}

	var groups []group
	threshold := 30.0 // colors within this distance are considered the same

	for _, c := range corners {
		found := false
		for i := range groups {
			if colorDistance(c, groups[i].representative) < threshold {
				groups[i].count++
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, group{representative: c, count: 1})
		}
	}

	// Find the group with the most members
	bestIdx := 0
	for i := range groups {
		if groups[i].count > groups[bestIdx].count {
			bestIdx = i
		}
	}

	return groups[bestIdx].representative
}
