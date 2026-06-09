# Image Resizer Pro (Go Edition) v0.5

High-performance, secure, and modern image processing suite built with Go and Gin-Gonic.

## Features
- **Transform:** 90/180/270° Rotation, Flip Horizontal/Vertical.
- **Resize:** Scale by percentage, dimensions, or Social Presets (Instagram, Facebook, Twitter, etc.).
- **Filters:** Grayscale, Invert, Blur, Sharpen, Sepia, and Pixelate.
- **Dynamic Adjustments:** Brightness, Contrast, Saturation with real-time sliders.
- **Advanced Tools:** Watermarking (Image/Text), Multi-file processing.
- **Metadata & Privacy:** Strip EXIF data with one click, Copyright embedding.
- **Productivity:** Batch ZIP bundling for all processed results.
- **PDF Support:** Convert multiple images into a single A4 PDF document.
- **Developer Ecosystem:** REST API with API Key authentication.

## Tech Stack
- **Backend:** Go 1.26.4+, Gin-Gonic
- **Processing Engine:** Imaging, Freetype, Gofpdf
- **Frontend:** Glassmorphism UI (Vanilla CSS + Vanilla JS)

## DevOps & Security
- **CI/CD:** Automated builds for GHCR.
- **Security:** Integrated `gosec` and `govulncheck` workflows.
- **Linting:** Automated `golangci-lint` verification.

## Running Locally
```bash
go run cmd/server/main.go
```
Open `http://localhost:5000` in your browser.

## API Usage
Refer to the **Developer API** section in the web interface for endpoint details and your API Key.
