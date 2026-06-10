package server

import (
	"archive/zip"
	"context"
	"fmt"
	"image-resizer/internal/config"
	"image-resizer/internal/middleware"
	"image-resizer/internal/processor"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
}

func (s *Server) handleIndex(c *gin.Context) {
	c.File("./web/templates/index.html")
}

// IMP-02 FIX: New route for downloading a single processed file with
// proper Content-Disposition header.
func (s *Server) handleDownload(c *gin.Context) {
	filename := filepath.Base(c.Param("filename")) // sanitize path component
	filePath := filepath.Join(s.cfg.ProcessedFolder, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "File not found")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.File(filePath)
}

func (s *Server) handleDownloadAll(c *gin.Context) {
	filenames := c.Query("files")
	if filenames == "" {
		c.String(http.StatusBadRequest, "No files specified")
		return
	}

	fileList := strings.Split(filenames, ",")
	zipName := fmt.Sprintf("processed_images_%d.zip", time.Now().Unix())
	zipPath := filepath.Join(s.cfg.ProcessedFolder, zipName)

	zipFile, err := os.Create(filepath.Clean(zipPath))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create zip")
		return
	}
	defer func() { _ = zipFile.Close() }()

	archive := zip.NewWriter(zipFile)
	defer func() { _ = archive.Close() }()

	for _, name := range fileList {
		safeName := filepath.Base(name)
		filePath := filepath.Join(s.cfg.ProcessedFolder, safeName)
		file, err := os.Open(filepath.Clean(filePath))
		if err != nil {
			continue
		}

		w, err := archive.Create(safeName)
		if err != nil {
			_ = file.Close()
			continue
		}

		if _, err := io.Copy(w, file); err != nil {
			_ = file.Close()
			continue
		}
		_ = file.Close()
	}

	// Must close writers before serving the file
	_ = archive.Close()
	_ = zipFile.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", zipName))
	c.File(zipPath)

}

func (s *Server) handleUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multipart form"})
		return
	}
	files, ok := form.File["files[]"]
	if !ok || len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	// BUG-06 FIX: Validate that all uploaded files have allowed image extensions.
	for _, file := range files {
		if !processor.IsAllowedImageFile(file.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("File type not allowed: %s. Allowed types: jpg, jpeg, png, gif, bmp, tiff, webp, ico", file.Filename),
			})
			return
		}
	}

	percentage, err := strconv.Atoi(c.DefaultPostForm("percentage", "100"))
	if err != nil || percentage <= 0 {
		percentage = 100
	}
	if percentage > 500 {
		percentage = 500
	}

	width, err := strconv.Atoi(c.DefaultPostForm("width", "0"))
	if err != nil {
		width = 0
	}
	height, err := strconv.Atoi(c.DefaultPostForm("height", "0"))
	if err != nil {
		height = 0
	}

	quality, err := strconv.Atoi(c.DefaultPostForm("quality", "85"))
	if err != nil || quality <= 0 || quality > 100 {
		quality = 85
	}

	// IMP-07 FIX: Validate rotation values — only 0, 90, 180, 270 are valid.
	rotation, err := strconv.Atoi(c.DefaultPostForm("rotation", "0"))
	if err != nil {
		rotation = 0
	}
	validRotations := map[int]bool{0: true, 90: true, 180: true, 270: true}
	if !validRotations[rotation] {
		rotation = 0
	}

	brightness, err := strconv.ParseFloat(c.DefaultPostForm("brightness", "0"), 64)
	if err != nil {
		brightness = 0
	}
	contrast, err := strconv.ParseFloat(c.DefaultPostForm("contrast", "0"), 64)
	if err != nil {
		contrast = 0
	}
	saturation, err := strconv.ParseFloat(c.DefaultPostForm("saturation", "0"), 64)
	if err != nil {
		saturation = 0
	}
	pixelate, err := strconv.Atoi(c.DefaultPostForm("pixelate", "0"))
	if err != nil {
		pixelate = 0
	}

	// BUG-08 FIX: Sanitize watermark filename to prevent path traversal.
	// Validate the watermark file is an image type before saving.
	watermarkFile, err := c.FormFile("watermark")
	watermarkPath := ""
	if err == nil {
		if !processor.IsAllowedImageFile(watermarkFile.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Watermark file type not allowed: %s", watermarkFile.Filename),
			})
			return
		}
		// Use filepath.Base to strip any directory components, then join safely
		safeWMName := filepath.Base(watermarkFile.Filename)
		watermarkPath = filepath.Join(s.cfg.UploadFolder, fmt.Sprintf("temp_watermark_%d_%s", time.Now().UnixMilli(), safeWMName))
		if err := c.SaveUploadedFile(watermarkFile, watermarkPath); err == nil {
			defer func() { _ = os.Remove(watermarkPath) }()
		} else {
			watermarkPath = ""
		}
	}

	// BUG-13 FIX: Support "fill" operation for social presets which uses
	// imaging.Fill (crop-to-fit) instead of imaging.Resize (stretch-to-fit).
	operation := c.PostForm("operation")
	if operation == "" {
		operation = "percentage"
	}

	opts := processor.ProcessOptions{
		Operation:      operation,
		Percentage:     percentage,
		Width:          width,
		Height:         height,
		Quality:        quality,
		Format:         c.PostForm("format"),
		Method:         c.PostForm("resize_method"),
		Rotation:       rotation,
		Flip:           c.PostForm("flip"),
		Filters:        c.PostFormArray("filters[]"),
		WatermarkPath:  watermarkPath,
		TextOverlay:    c.PostForm("text_overlay"),
		TextColor:      c.PostForm("text_color"),
		TextSize:       0, // will use default in processor
		StripEXIF:      c.PostForm("strip_exif") == "on",
		Copyright:      c.PostForm("copyright"),
		Brightness:     brightness,
		Contrast:       contrast,
		Saturation:     saturation,
		Pixelate:       pixelate,
		Crop:           c.PostForm("crop"),
		Vignette:       c.PostForm("vignette") == "on",
		RenameTemplate: c.PostForm("rename_template"),
	}

	var results []processor.ProcessResult
	var processedPaths []string
	var errors []string

	for _, file := range files {
		filename := filepath.Base(file.Filename)
		uploadPath := filepath.Join(s.cfg.UploadFolder, filename)

		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			// IMP-01 FIX: Collect errors instead of silently continuing
			errors = append(errors, fmt.Sprintf("failed to save %s: %v", filename, err))
			continue
		}

		res, err := processor.ProcessImage(uploadPath, s.cfg.ProcessedFolder, &opts)
		_ = os.Remove(uploadPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to process %s: %v", filename, err))
			continue
		}

		results = append(results, *res)
		processedPaths = append(processedPaths, res.NewFilePath)
	}

	// BUG-04 FIX (reverted): When PDF output is requested, replace the results array
	// with just the PDF document instead of returning both the PDF and intermediate JPGs.
	// This prevents the frontend from displaying intermediate JPGs to the user.
	if opts.Format == "pdf" && len(processedPaths) > 0 {
		pdfPath := filepath.Join(s.cfg.ProcessedFolder, fmt.Sprintf("document_%d.pdf", time.Now().UnixMilli()))
		if err := processor.CreatePDF(processedPaths, pdfPath); err == nil {
			pdfResult := processor.ProcessResult{
				OriginalName:  "Multiple Images",
				ProcessedName: filepath.Base(pdfPath),
				OriginalSize:  "N/A",
				NewSize:       "PDF Document",
				NewFilePath:   pdfPath,
			}
			results = []processor.ProcessResult{pdfResult}
		} else {
			errors = append(errors, fmt.Sprintf("failed to create PDF: %v", err))
		}
	}

	// IMP-01 FIX: Return error details in the response instead of silently
	// returning empty arrays. If all files failed, return 207 Multi-Status.
	if len(results) == 0 && len(errors) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "All files failed to process",
			"errors": errors,
		})
		return
	}

	response := gin.H{
		"results": results,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
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
