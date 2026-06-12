package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"image-resizer/internal/config"
	"image-resizer/internal/middleware"
	"image-resizer/internal/processor"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
	cfg    *config.Config
	http   *http.Server
}

func NewServer(cfg *config.Config) *Server {
	r := gin.Default()

	// IMP-03 FIX: Enforce MaxContentLength limit that was defined but never used.
	r.MaxMultipartMemory = cfg.MaxContentLength

	s := &Server{
		router: r,
		cfg:    cfg,
		http: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	s.setupRoutes()
	go s.startCleanupWorker()

	return s
}

func (s *Server) setupRoutes() {
	s.router.GET("/", s.handleIndex)
	s.router.GET("/favicon.ico", func(c *gin.Context) {
		c.File("./web/static/assets/logo.svg")
	})
	s.router.POST("/", s.handleUpload)
	s.router.GET("/download-all", s.handleDownloadAll)

	// IMP-02 FIX: Add single file download route with Content-Disposition header
	// so direct URL access triggers a download with the correct filename.
	s.router.GET("/download/:filename", s.handleDownload)

	// Private download with signed URLs (Item 99)
	s.router.GET("/private/download/:filename", s.handlePrivateDownload)

	s.router.Static("/static", "./web/static")
	s.router.Static("/processed", s.cfg.ProcessedFolder)

	// Developer API
	// IMP-05 FIX: Add CORS middleware to API routes so browser-based
	// cross-origin clients can call the API endpoints.
	api := s.router.Group("/api/v1")
	api.Use(middleware.CORS())
	api.Use(middleware.APIKeyAuth(s.cfg.APIKey))

	api.POST("/process", s.handleUpload)
	api.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "operational", "version": "v2.0.0-go"})
	})
	api.GET("/presets", func(c *gin.Context) {
		c.JSON(http.StatusOK, processor.SocialPresets)
	})
	// Private link generation (Item 99)
	api.POST("/generate-link", s.handleGenerateLink)
	// Decryption endpoint (Item 100)
	api.POST("/decrypt", s.handleDecrypt)
}

func (s *Server) startCleanupWorker() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		s.cleanupFiles(s.cfg.UploadFolder, 1*time.Hour)
		s.cleanupFiles(s.cfg.ProcessedFolder, 12*time.Hour)
	}
}

func (s *Server) cleanupFiles(dir string, maxAge time.Duration) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, f.Name()))
		}
	}
}

// Start begins listening on the configured port.
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// IMP-04 FIX: Graceful shutdown support. Previously, the server used
// gin.Run() which doesn't handle OS signals, causing abrupt termination
// of in-progress requests when the container stops.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
