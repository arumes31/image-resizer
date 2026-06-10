# <img src="web/static/assets/logo.svg" width="48" height="48" valign="middle"> Image Resizer Pro (Go Edition)

![Build Status](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/build.yml?branch=v0.5&style=for-the-badge&logo=github&label=Build)
![Security Audit](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/security.yml?branch=v0.5&style=for-the-badge&logo=pre-commit&label=Security)
![Linting](https://img.shields.io/github/actions/workflow/status/arumes31/image-resizer/lint.yml?branch=v0.5&style=for-the-badge&logo=go&label=Lint)
![Go Version](https://img.shields.io/badge/Go-1.26.4-00ADD8?style=for-the-badge&logo=go)
![Docker Ready](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker)

**High-performance, secure, and modern image processing suite built with Go and Gin-Gonic.**

---

## 🚀 Key Features

| 🎭 **Artistic** | 📐 **Transform** | 🛠️ **Utility** |
| :--- | :--- | :--- |
| **Filters:** Noir, Vivid, Sepia, Invert, Grayscale | **Smart Crop:** 1:1, 16:9, 4:3 | **Batch Rename:** Custom templates |
| **Adjustments:** Brightness, Contrast, Saturation | **Resize:** % or Pixel accurate | **Zip Bundling:** Batch downloads |
| **Effects:** Pixelate, Blur, Sharpen | **Rotate:** 90°, 180°, 270° | **PDF Export:** Images to Document |
| **Overlays:** Text (Custom Font), Image | **Flip:** Horizontal & Vertical | **EXIF Strip:** Privacy protection |

---

## 📊 Performance Benchmark

```mermaid
xychart-beta
    title "Processing Speed: Go vs Python (100MB Batch)"
    x-axis ["Python (Pillow)", "Go (Imaging)"]
    y-axis "Time (Seconds)" 0 --> 20
    bar [18.4, 4.2]
```
*Note: The figures shown above are illustrative/approximate.*

---

## 🏗️ Architecture

```mermaid
graph TD
    UI[Web UI / JS] -->|Multipart Form| API[Go API / Gin]
    API -->|Auth Middleware| SEC[Security Layer]
    SEC -->|Options| PROC[Image Processor]
    PROC -->|Imaging| RES[Resizer/Filters]
    PROC -->|Freetype| TXT[Text Overlay]
    PROC -->|Gofpdf| PDF[PDF Generator]
    RES -->|Save| STORAGE[static/processed]
    PDF -->|Save| STORAGE
    STORAGE -->|Download| USER[User]
```

---

## 🛠️ Tech Stack

- **Backend:** [Go 1.26.4+](https://go.dev/) + [Gin-Gonic](https://gin-gonic.com/)
- **Processing:** [Imaging](https://github.com/disintegration/imaging), [Freetype](https://github.com/golang/freetype), [Gofpdf](https://github.com/jung-kurt/gofpdf)
- **Frontend:** Glassmorphism UI (Vanilla CSS + Vanilla JS)
- **DevOps:** GitHub Actions, Docker (Alpine Multi-stage), [gosec](https://github.com/securego/gosec), [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)

---

## 🚦 Quick Start

### 📦 Run with Docker
```bash
docker pull ghcr.io/arumes31/image-resizer:latest
docker run -p 5000:5000 ghcr.io/arumes31/image-resizer:latest
```

### 🔨 Development Mode
```bash
go run cmd/server/main.go
```
Open `http://localhost:5000` to access the dashboard.

---

## 🧑‍💻 Developer API

The suite includes a secure REST API for headless automation.

**Authentication:**
Required Header: `X-API-Key: your_secure_key`

**Endpoints:**
- `POST /api/v1/process`: Upload and process images (supports all dashboard features).
- `GET /api/v1/status`: Check system health and version.

---

## 🔒 Security & Privacy
- **Privacy:** One-click EXIF stripping removes metadata (GPS, Timestamps).
- **Safety:** Constant-time comparison for API keys; 500% scaling cap to prevent memory exhaustion (enforced in `handleUpload` in `internal/server/server.go`).
- **Audited:** Scanned with `gosec` and `govulncheck` on every commit.

---
*Built with ❤️ by arumes31 - 2026 Edition*
