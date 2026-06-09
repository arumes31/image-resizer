package server

import (
	"archive/zip"
	"fmt"
	"image-resizer/internal/config"
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

	zipFile, err := os.Create(zipPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create zip")
		return
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	for _, name := range fileList {
		filePath := filepath.Join(s.cfg.ProcessedFolder, name)
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		defer file.Close()

		w, err := archive.Create(name)
		if err != nil {
			continue
		}

		if _, err := io.Copy(w, file); err != nil {
			continue
		}
	}

	archive.Close()
	zipFile.Close()

	c.File(zipPath)
	// Optionally delete the zip after sending, but cleanup worker will handle it
}

func (s *Server) handleUpload(c *gin.Context) {
	form, _ := c.MultipartForm()
	files := form.File["files[]"]
	
	percentage, _ := strconv.Atoi(c.PostForm("percentage"))
	width, _ := strconv.Atoi(c.PostForm("width"))
	height, _ := strconv.Atoi(c.PostForm("height"))
	quality, _ := strconv.Atoi(c.PostForm("quality"))
	rotation, _ := strconv.Atoi(c.PostForm("rotation"))
	
	watermarkFile, err := c.FormFile("watermark")
	watermarkPath := ""
	if err == nil {
		watermarkPath = filepath.Join(s.cfg.UploadFolder, "temp_watermark_" + watermarkFile.Filename)
		c.SaveUploadedFile(watermarkFile, watermarkPath)
		defer os.Remove(watermarkPath)
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
			continue
		}

		os.Remove(uploadPath)
		results = append(results, *res)
		processedPaths = append(processedPaths, res.NewFilePath)
	}

	// If format is PDF, create a single PDF and return it as the first result
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
			os.Remove(filepath.Join(dir, f.Name()))
		}
	}
}

func (s *Server) Start() error {
	return s.router.Run(":" + s.cfg.Port)
}
