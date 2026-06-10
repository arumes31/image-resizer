package server

import (
	"archive/zip"
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
}

func NewServer(cfg *config.Config) *Server {
	r := gin.Default()
	
	s := &Server{
		router: r,
		cfg:    cfg,
	}

	s.setupRoutes()
	go s.startCleanupWorker()

	return s
}

func (s *Server) setupRoutes() {
	s.router.GET("/", s.handleIndex)
	s.router.POST("/", s.handleUpload)
	s.router.GET("/download-all", s.handleDownloadAll)
	s.router.Static("/static", "./web/static")
	s.router.Static("/processed", s.cfg.ProcessedFolder)

	// Developer API
	api := s.router.Group("/api/v1")
	api.Use(middleware.APIKeyAuth(s.cfg.APIKey))
	{
		api.POST("/process", s.handleUpload)
		api.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "operational", "version": "v2.0.0-go"})
		})
	}
}

func (s *Server) handleIndex(c *gin.Context) {
	c.File("./web/templates/index.html")
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

	_ = archive.Close()
	_ = zipFile.Close()

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
	
	percentage, _ := strconv.Atoi(c.DefaultPostForm("percentage", "100"))
	if percentage <= 0 { percentage = 100 }
	if percentage > 500 { percentage = 500 }
	
	width, _ := strconv.Atoi(c.DefaultPostForm("width", "0"))
	height, _ := strconv.Atoi(c.DefaultPostForm("height", "0"))
	
	quality, _ := strconv.Atoi(c.DefaultPostForm("quality", "100"))
	if quality <= 0 || quality > 100 { quality = 100 }
	
	rotation, _ := strconv.Atoi(c.DefaultPostForm("rotation", "0"))
	brightness, _ := strconv.ParseFloat(c.DefaultPostForm("brightness", "0"), 64)
	contrast, _ := strconv.ParseFloat(c.DefaultPostForm("contrast", "0"), 64)
	saturation, _ := strconv.ParseFloat(c.DefaultPostForm("saturation", "0"), 64)
	pixelate, _ := strconv.Atoi(c.DefaultPostForm("pixelate", "0"))
	
	watermarkFile, err := c.FormFile("watermark")
	watermarkPath := ""
	if err == nil {
		watermarkPath = filepath.Join(s.cfg.UploadFolder, "temp_watermark_" + filepath.Base(watermarkFile.Filename))
		if err := c.SaveUploadedFile(watermarkFile, watermarkPath); err == nil {
			defer func() { _ = os.Remove(watermarkPath) }()
		}
	}

	opts := processor.ProcessOptions{
		Operation:  c.PostForm("operation"),
		Percentage: percentage,
		Width:      width,
		Height:     height,
		Quality:    quality,
		Format:     c.PostForm("format"),
		Method:     c.PostForm("resize_method"),
		Rotation:   rotation,
		Flip:       c.PostForm("flip"),
		Filters:    c.PostFormArray("filters[]"),
		WatermarkPath: watermarkPath,
		TextOverlay:   c.PostForm("text_overlay"),
		StripEXIF:     c.PostForm("strip_exif") == "on",
		Copyright:     c.PostForm("copyright"),
		Brightness:    brightness,
		Contrast:      contrast,
		Saturation:    saturation,
		Pixelate:      pixelate,
		Crop:          c.PostForm("crop"),
		RenameTemplate: c.PostForm("rename_template"),
	}

	var results []processor.ProcessResult
	var processedPaths []string

	for _, file := range files {
		filename := filepath.Base(file.Filename)
		uploadPath := filepath.Join(s.cfg.UploadFolder, filename)
		
		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			continue
		}

		res, err := processor.ProcessImage(uploadPath, s.cfg.ProcessedFolder, opts)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", filename, err)
			_ = os.Remove(uploadPath)
			continue
		}

		_ = os.Remove(uploadPath)
		results = append(results, *res)
		processedPaths = append(processedPaths, res.NewFilePath)
	}

	if opts.Format == "pdf" && len(processedPaths) > 0 {
		pdfPath := filepath.Join(s.cfg.ProcessedFolder, fmt.Sprintf("document_%d.pdf", time.Now().Unix()))
		if err := processor.CreatePDF(processedPaths, pdfPath); err == nil {
			pdfResult := processor.ProcessResult{
				OriginalName:  "Multiple Images",
				ProcessedName: filepath.Base(pdfPath),
				OriginalSize:  "N/A",
				NewSize:       "PDF Document",
				NewFilePath:   pdfPath,
			}
			c.JSON(http.StatusOK, []processor.ProcessResult{pdfResult})
			return
		}
	}

	c.JSON(http.StatusOK, results)
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

func (s *Server) Start() error {
	return s.router.Run(":" + s.cfg.Port)
}
