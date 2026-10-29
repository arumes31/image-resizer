package server

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"image-resizer/internal/config"
	"image-resizer/internal/middleware"
	"image-resizer/internal/processor"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Limits for /download-all archives.
const (
	// maxArchiveFiles is the most distinct files one archive may contain.
	maxArchiveFiles = 100
	// maxArchiveBytes caps the combined size of the files in one archive.
	maxArchiveBytes = 512 << 20
	// maxConcurrentArchives bounds archives built at the same time.
	maxConcurrentArchives = 2
)

// archiveName matches the names ProcessImage gives its outputs.
var archiveName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

type Server struct {
	router   *gin.Engine
	cfg      *config.Config
	http     *http.Server
	jobs     chan struct{} // bounds concurrent image processing
	archives chan struct{} // bounds concurrent archive builds
}

func NewServer(cfg *config.Config) *Server {
	r := gin.Default()

	// Only trust X-Forwarded-For from explicitly configured proxies, so
	// clients cannot spoof their IP to evade per-IP rate limiting.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("FATAL: invalid TRUSTED_PROXIES: %v", err)
	}

	// IMP-03 FIX: Enforce MaxContentLength limit that was defined but never used.
	// The hard request body cap is applied with http.MaxBytesReader in handleUpload.
	r.MaxMultipartMemory = cfg.MaxContentLength

	maxJobs := cfg.MaxConcurrent
	if maxJobs <= 0 {
		maxJobs = runtime.NumCPU()
	}

	s := &Server{
		router:   r,
		cfg:      cfg,
		jobs:     make(chan struct{}, maxJobs),
		archives: make(chan struct{}, maxConcurrentArchives),
		http: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	s.setupRoutes()
	go s.startCleanupWorker()

	return s
}

func (s *Server) setupRoutes() {
	var uploadHandlers []gin.HandlerFunc
	if s.cfg.RateLimitPerMinute > 0 {
		uploadHandlers = append(uploadHandlers, middleware.NewRateLimiter(s.cfg.RateLimitPerMinute).Middleware())
	}
	uploadHandlers = append(uploadHandlers, s.handleUpload)

	// Web UI: optional HTTP Basic auth (UI_PASSWORD), CSRF check and rate limit
	// on uploads. Without UI_PASSWORD the UI stays open as before.
	ui := s.router.Group("/", middleware.UIBasicAuth(s.cfg.UIUsername, s.cfg.UIPassword))
	ui.GET("/", s.handleIndex)
	ui.POST("/", append([]gin.HandlerFunc{middleware.SameOrigin()}, uploadHandlers...)...)
	ui.GET("/download-all", s.handleDownloadAll)

	// IMP-02 FIX: Add single file download route with Content-Disposition header
	// so direct URL access triggers a download with the correct filename.
	ui.GET("/download/:filename", s.handleDownload)
	ui.Static("/processed", s.cfg.ProcessedFolder)

	s.router.Static("/static", "./web/static")

	// Developer API
	// IMP-05 FIX: Add CORS middleware to API routes so browser-based
	// cross-origin clients can call the API endpoints.
	api := s.router.Group("/api/v1")
	api.Use(middleware.CORS())
	api.Use(middleware.APIKeyAuth(s.cfg.APIKey))

	api.POST("/process", uploadHandlers...)
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

	// All file access goes through an os.Root, which refuses names that
	// escape the processed folder (including via symlinks).
	root, rErr := os.OpenRoot(s.cfg.ProcessedFolder)
	if rErr != nil {
		c.String(http.StatusInternalServerError, "Failed to create zip")
		return
	}
	defer func() { _ = root.Close() }()

	// Deduplicate names and size the archive before doing any work, so a
	// short query cannot make the server copy the same large file repeatedly.
	var members []string
	seen := make(map[string]bool)
	var totalBytes int64
	for _, name := range strings.Split(filenames, ",") {
		safeName := filepath.Base(name)
		if !archiveName.MatchString(safeName) || seen[safeName] {
			continue
		}
		seen[safeName] = true
		info, err := root.Stat(safeName)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		totalBytes += info.Size()
		members = append(members, safeName)
	}
	if len(members) > maxArchiveFiles {
		c.String(http.StatusRequestEntityTooLarge, fmt.Sprintf("Too many files (max %d per archive)", maxArchiveFiles))
		return
	}
	if totalBytes > maxArchiveBytes {
		c.String(http.StatusRequestEntityTooLarge, fmt.Sprintf("Archive too large (max %d MB)", maxArchiveBytes>>20))
		return
	}

	ctx := c.Request.Context()
	select {
	case s.archives <- struct{}{}:
		defer func() { <-s.archives }()
	case <-ctx.Done():
		return
	}

	zipName := fmt.Sprintf("processed_images_%d.zip", time.Now().Unix())

	// Build the archive in a unique temp file so concurrent requests never
	// collide; it is removed once it has been served.
	zipFile, err := os.CreateTemp("", "processed_images_*.zip")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create zip")
		return
	}
	zipPath := zipFile.Name()
	defer func() { _ = os.Remove(zipPath) }()
	defer func() { _ = zipFile.Close() }()

	archive := zip.NewWriter(zipFile)
	defer func() { _ = archive.Close() }()

	for _, name := range members {
		if ctx.Err() != nil {
			return
		}
		file, err := root.Open(name)
		if err != nil {
			continue
		}

		// Processed images are already compressed; storing them avoids
		// burning CPU on a second compression pass.
		w, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			_ = file.Close()
			continue
		}

		_, err = io.Copy(w, &ctxReader{ctx: ctx, r: file})
		_ = file.Close()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			continue
		}
	}

	// Must close writers before serving the file
	_ = archive.Close()
	_ = zipFile.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", zipName))
	c.File(zipPath)

}

// ctxReader stops a copy as soon as its context is canceled.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// publicError maps a processing error to a message that is safe to return to
// clients. Unexpected errors are logged server-side and reported generically.
func publicError(err error) string {
	for _, known := range []error{
		processor.ErrImageTooLarge,
		processor.ErrOutputTooLarge,
		processor.ErrInvalidImage,
		processor.ErrInvalidFormat,
		processor.ErrInvalidRenameTemplate,
	} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "internal processing error"
}

func (s *Server) handleUpload(c *gin.Context) {
	// Hard cap on the request body; MaxMultipartMemory alone only limits
	// in-memory buffering and lets larger parts spill to disk.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.MaxContentLength)

	form, err := c.MultipartForm()
	if err != nil || form == nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("Request too large (max %d MB)", s.cfg.MaxContentLength/(1024*1024)),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multipart form"})
		return
	}
	defer func() {
		if rErr := form.RemoveAll(); rErr != nil {
			log.Printf("failed to remove multipart temp files: %v", rErr)
		}
	}()

	files, ok := form.File["files[]"]
	if !ok || len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}
	if s.cfg.MaxFilesPerRequest > 0 && len(files) > s.cfg.MaxFilesPerRequest {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Too many files (max %d per request)", s.cfg.MaxFilesPerRequest),
		})
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
	if err != nil || width < 0 {
		width = 0
	}
	width = min(width, processor.MaxOutputDimension)
	height, err := strconv.Atoi(c.DefaultPostForm("height", "0"))
	if err != nil || height < 0 {
		height = 0
	}
	height = min(height, processor.MaxOutputDimension)

	format := strings.ToLower(c.PostForm("format"))
	if !processor.IsAllowedFormat(format) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Output format not allowed"})
		return
	}
	renameTemplate := c.PostForm("rename_template")
	if vErr := processor.ValidateRenameTemplate(renameTemplate); vErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": vErr.Error()})
		return
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
		// Store under a random name so concurrent uploads never collide.
		token, tErr := processor.RandomToken()
		if tErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal processing error"})
			return
		}
		wmExt := strings.ToLower(filepath.Ext(watermarkFile.Filename))
		watermarkPath = filepath.Join(s.cfg.UploadFolder, "temp_watermark_"+token+wmExt)
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
		Format:         format,
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
		RenameTemplate: renameTemplate,
	}

	var results []processor.ProcessResult
	var processedPaths []string
	var errs []string

	for _, file := range files {
		filename := processor.SanitizeFileName(file.Filename)

		// Uploads are stored under random names so that users who upload
		// files with the same name never overwrite each other.
		token, err := processor.RandomToken()
		if err != nil {
			log.Printf("failed to generate upload name: %v", err)
			errs = append(errs, fmt.Sprintf("failed to save %s", filename))
			continue
		}
		uploadPath := filepath.Join(s.cfg.UploadFolder, token+strings.ToLower(filepath.Ext(file.Filename)))

		if sErr := c.SaveUploadedFile(file, uploadPath); sErr != nil {
			// IMP-01 FIX: Collect errors instead of silently continuing
			log.Printf("failed to save upload %s: %v", filename, sErr)
			errs = append(errs, fmt.Sprintf("failed to save %s", filename))
			continue
		}

		// Bound the number of images decoded at once across all requests.
		select {
		case s.jobs <- struct{}{}:
		case <-c.Request.Context().Done():
			_ = os.Remove(uploadPath)
			return
		}
		res, err := processor.ProcessImage(c.Request.Context(), uploadPath, file.Filename, s.cfg.ProcessedFolder, &opts)
		<-s.jobs
		_ = os.Remove(uploadPath)
		if c.Request.Context().Err() != nil {
			// The client is gone; skip the remaining files.
			return
		}
		if err != nil {
			log.Printf("failed to process %s: %v", filename, err)
			errs = append(errs, fmt.Sprintf("failed to process %s: %s", filename, publicError(err)))
			continue
		}

		results = append(results, *res)
		processedPaths = append(processedPaths, res.NewFilePath)
	}

	// BUG-04 FIX (reverted): When PDF output is requested, replace the results array
	// with just the PDF document instead of returning both the PDF and intermediate JPGs.
	// This prevents the frontend from displaying intermediate JPGs to the user.
	if opts.Format == "pdf" && len(processedPaths) > 0 {
		token, err := processor.RandomToken()
		if err != nil {
			token = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		pdfPath := filepath.Join(s.cfg.ProcessedFolder, "document_"+token+".pdf")
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
			log.Printf("failed to create PDF: %v", err)
			errs = append(errs, "failed to create PDF")
		}
	}

	// IMP-01 FIX: Return error details in the response instead of silently
	// returning empty arrays. If all files failed, return 207 Multi-Status.
	if len(results) == 0 && len(errs) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "All files failed to process",
			"errors": errs,
		})
		return
	}

	response := gin.H{
		"results": results,
	}
	if len(errs) > 0 {
		response["errors"] = errs
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

// Start begins listening on the configured port, serving HTTPS when a
// certificate and key are configured.
func (s *Server) Start() error {
	if s.cfg.TLSCertFile != "" {
		return s.http.ListenAndServeTLS(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	}
	return s.http.ListenAndServe()
}

// IMP-04 FIX: Graceful shutdown support. Previously, the server used
// gin.Run() which doesn't handle OS signals, causing abrupt termination
// of in-progress requests when the container stops.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
