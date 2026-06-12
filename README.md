# <img src="web/static/assets/logo.svg" width="48" height="48" valign="middle"> NanoBanana — Image Processing Toolkit

![Build Status](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/build.yml?branch=v0.5&style=for-the-badge&logo=github&label=Build)
![Security Audit](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/security.yml?branch=v0.5&style=for-the-badge&logo=pre-commit&label=Security)
![Linting](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/lint.yml?branch=v0.5&style=for-the-badge&logo=go&label=Lint)
![Go Version](https://img.shields.io/badge/Go-1.26.4-00ADD8?style=for-the-badge&logo=go)
![Docker Ready](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker)

**Comprehensive, secure image processing suite built with Go & Gin. From basic resizing to professional-grade adjustments, background removal, social media optimization, steganography, and encrypted sharing — all in one self-contained service with a modern Glassmorphism UI.**

---

## ✨ Features

### 🖼️ Background Removal
| Feature | Description |
| :--- | :--- |
| Transparent removal | Auto-detect background from corner pixels |
| Flood-fill from edges | Remove contiguous background color from edges |
| Color-match removal | Target a specific color with configurable tolerance |
| Edge smoothing | Feather/smooth edges for clean cutouts |

### 📁 Format Support & Encoding
| Feature | Notes |
| :--- | :--- |
| AVIF | Requires CGO + libaom |
| HEIC / HEIF | Requires CGO + libheif |
| JPEG XL | Stub — external tools needed |
| SVG rasterization | Pure Go via oksvg |
| ICO multi-size bundler | 16×16 to 256×256 |
| Animated GIF | Full decode/encode support |
| RAW files | Requires CGO + libraw |
| Base64 export | With `data:` URI prefix |
| Progressive JPEG | Toggle on/off |
| TIFF | Multi-page extraction |
| PDF-to-image | Requires external tools (poppler) |
| HDR → SDR | Reinhard tone-mapping operator |
| Web Manifest | PWA manifest generator |
| Lossless WebP | Optimized encoding |
| DICOM | 8/16-bit grayscale + RGB |

### 🎨 Professional Adjustments
| Feature | Details |
| :--- | :--- |
| HSL sliders | Hue, Saturation, Lightness |
| Curves | 5 presets + cubic spline interpolation |
| Levels | Black point, white point, gamma |
| Selective color | 4 presets per hue range |
| Chromatic aberration fix | Automatic correction |
| Unsharp mask | Amount + radius control |
| Film grain | Realistic simulation |
| Temperature & tint | White balance control |
| Shadow / highlight | Selective recovery |
| Vignette | Amount, feather, roundness, midpoint |

### 🏷️ Branding & Overlays
| Feature | Details |
| :--- | :--- |
| Dynamic text watermarks | `{filename}`, `{date}`, `{time}`, `{year}` tokens |
| Tiled watermarks | Diagonal repeat pattern |
| QR code overlay | Custom content + positioning |
| Barcode generator | Code128, EAN-13 |
| Rounded corners | Configurable radius |
| Drop shadows | Offset, blur, color |
| Stroke / border | Solid or dashed |
| Placeholder generator | Custom dimensions + text |
| Steganography | Invisible LSB encoding |
| Signature stamp | Overlay placement |

### 📱 Social Media Optimization
| Feature | Details |
| :--- | :--- |
| Instagram carousel slicer | 1080×1350 slides |
| YouTube thumbnail | Safe-zone overlay |
| Discord / Slack emoji | Auto file-size reduction |
| App store screenshots | Device frame overlay |
| LinkedIn banner | 1584×396 preset |
| Twitter / X header | 1500×500 optimization |
| Pinterest long-pin | Multi-image stitcher |
| Favicon generator | 10+ sizes + `manifest.json` |
| Twitch panel templates | Ready-made layouts |
| 19 social presets | One-click optimization |

### 🔐 Privacy, Security & Ethics
| Feature | Details |
| :--- | :--- |
| Zero-log mode | In-memory processing, no disk writes |
| Private link sharing | HMAC-signed 24 h self-destructing URLs |
| Local-first encryption | AES-256-GCM with password |

### 🎭 Core Processing
| Feature | Details |
| :--- | :--- |
| Smart crop | 1:1, 16:9, 4:3 aspect ratios |
| Resize | Percentage or pixel-accurate |
| Rotate / Flip | 90°, 180°, 270° + H/V flip |
| Artistic filters | Noir, Vivid, Sepia, Invert, Grayscale, Pixelate, Blur, Sharpen |
| Basic adjustments | Brightness, Contrast, Saturation |
| PDF export | Images to document |
| EXIF strip | One-click metadata removal |
| Batch rename | Custom templates |
| ZIP bundling | Batch downloads |

---

## 🏗️ Architecture

```
internal/processor/
├── processor.go      # Pipeline orchestrator, ProcessOptions, shared utilities
├── transforms.go     # Crop, rotation, flip
├── filters.go       # Artistic filters (8 filters)
├── adjustments.go   # Basic + professional adjustments (HSL, curves, levels, etc.)
├── overlays.go      # Watermarks, text, QR codes, borders, shadows, steganography
├── codecs.go        # Format-specific save logic, alpha compositing
├── background.go    # Background removal (3 methods)
├── social.go        # Social media optimization (carousel, safe zones, favicons)
├── formats.go       # Format-specific decode (SVG, DICOM, TIFF, animated GIF)
├── presets.go       # Social media preset definitions
├── crypto.go        # AES-GCM encryption/decryption
├── avif_cgo.go      # AVIF support (CGO build)
├── avif_nocgo.go    # AVIF stub (non-CGO build)
├── heic_cgo.go      # HEIC support (CGO build)
├── heic_nocgo.go    # HEIC stub (non-CGO build)
├── jxl.go           # JPEG XL stub
├── raw_cgo.go       # RAW support (CGO build)
└── raw_nocgo.go     # RAW stub (non-CGO build)
```

---

## 🚦 Quick Start

### 📦 Run with Docker

```bash
docker pull ghcr.io/arumes31/image-resizer:latest
docker run -p 5000:5000 ghcr.io/arumes31/image-resizer:latest
```

**With CGO dependencies** (AVIF / HEIC / RAW support):

```bash
docker build --build-arg CGO_ENABLED=1 -t image-resizer:cgo .
docker run -p 5000:5000 image-resizer:cgo
```

### 🔨 Development Mode

```bash
go run cmd/server/main.go
```

Open `http://localhost:5000` to access the dashboard.

---

## 🧑‍💻 API Documentation

All API endpoints (except `/api/v1/status`) require an API key via the `X-API-Key` header.

| Method | Path | Auth | Description |
| :--- | :--- | :--- | :--- |
| `POST` | `/` | No | Web UI upload |
| `POST` | `/api/v1/process` | Yes | Image processing pipeline |
| `GET` | `/api/v1/status` | No | Health check & version |
| `GET` | `/api/v1/presets` | Yes | List social media presets |
| `POST` | `/api/v1/generate-link` | Yes | Generate private download link |
| `POST` | `/api/v1/decrypt` | Yes | Decrypt encrypted output file |
| `GET` | `/private/download/:filename` | Signed | HMAC-signed download URL |
| `GET` | `/download/:filename` | No | Direct download |
| `GET` | `/download-all` | No | ZIP bundle download |

### Process Images — `POST /api/v1/process`

**Content-Type:** `multipart/form-data`

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `files[]` | File | — | **(Required)** Images to process |
| `operation` | String | `percentage` | `percentage` or `fill` |
| `percentage` | Integer | `100` | Scale percentage (max 500) |
| `width` | Integer | `0` | Target width in pixels |
| `height` | Integer | `0` | Target height in pixels |
| `quality` | Integer | `85` | Output quality (1–100) |
| `rotation` | Integer | `0` | `0`, `90`, `180`, `270` |
| `brightness` | Float | `0` | Brightness adjustment |
| `contrast` | Float | `0` | Contrast adjustment |
| `saturation` | Float | `0` | Saturation adjustment |
| `pixelate` | Integer | `0` | Pixelation factor |
| `watermark` | File | — | Image watermark overlay |
| `format` | String | Original | Output format (`jpg`, `png`, `webp`, `avif`, `pdf`, …) |
| `flip` | String | — | `horizontal` or `vertical` |
| `filters[]` | String[] | — | `Noir`, `Vivid`, `Sepia`, `Invert`, `Grayscale` |
| `text_overlay` | String | — | Text to render on image |
| `text_color` | String | — | Hex color for text overlay |
| `strip_exif` | String | — | `on` to remove EXIF metadata |
| `copyright` | String | — | Copyright text in metadata |
| `crop` | String | — | Crop aspect ratio |
| `vignette` | String | — | `on` to apply vignette |
| `rename_template` | String | — | Output file rename template |

**Success response (200):**

```json
{
  "results": [
    {
      "OriginalName": "photo.jpg",
      "ProcessedName": "processed_photo.jpg",
      "OriginalSize": "1.5 MB",
      "NewSize": "450 KB",
      "NewFilePath": "static/processed/processed_photo.jpg"
    }
  ]
}
```

---

## ⚙️ Configuration

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `5000` | Server port |
| `API_KEY` | — | API authentication key |
| `ENV` | `development` | `development` or `production` |
| `SIGNING_KEY` | auto-generated | HMAC key for private links |
| `LINK_EXPIRY` | `24` | Private link expiry (hours) |
| `UPLOAD_FOLDER` | `static/uploads` | Temp upload directory |
| `PROCESSED_FOLDER` | `static/processed` | Output directory |

---

## 🏷️ Build Tags (CGO)

Some formats require C libraries and are gated behind CGO build tags:

| Format | Build Tag | Dependency |
| :--- | :--- | :--- |
| AVIF | `avif` | libaom |
| HEIC / HEIF | `heic` | libheif |
| RAW | `raw` | libraw |

**Build with all CGO formats:**

```bash
CGO_ENABLED=1 go build -tags "avif,heic,raw" -o image-resizer cmd/server/main.go
```

**Build without CGO (pure Go):**

```bash
CGO_ENABLED=0 go build -o image-resizer cmd/server/main.go
```

Without CGO, AVIF/HEIC/RAW features return a friendly stub error at runtime.

---

## 🛠️ Tech Stack

- **Backend:** [Go 1.26.4+](https://go.dev/) + [Gin-Gonic](https://gin-gonic.com/)
- **Processing:** [Imaging](https://github.com/disintegration/imaging), [Freetype](https://github.com/golang/freetype), [Gofpdf](https://github.com/jung-kurt/gofpdf), [oksvg](https://github.com/ajstarks/oksvg)
- **Frontend:** Glassmorphism UI (Vanilla CSS + Vanilla JS)
- **DevOps:** GitHub Actions, Docker (Alpine multi-stage), [gosec](https://github.com/securego/gosec), [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)

---

## 🔒 Security & Privacy

- **EXIF stripping** — One-click removal of GPS, timestamps, and camera metadata
- **API-key auth** — Constant-time comparison; 500 % scaling cap prevents memory exhaustion
- **Zero-log mode** — In-memory processing with no disk writes
- **Encrypted output** — AES-256-GCM with password-derived key
- **Private links** — HMAC-signed, time-limited, self-destructing download URLs
- **Audited** — Scanned with `gosec` and `govulncheck` on every commit

---

*Built with ❤️ by arumes31 — 2026 Edition*
