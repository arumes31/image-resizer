package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"image-resizer/internal/middleware"
	"image-resizer/internal/processor"

	"github.com/gin-gonic/gin"
)

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

	// Background Removal fields
	bgRemovalTolerance, _ := strconv.Atoi(c.DefaultPostForm("bg_removal_tolerance", "30"))
	bgRemovalEdgeSmooth, _ := strconv.Atoi(c.DefaultPostForm("bg_removal_edge_smooth", "2"))

	// Professional Adjustments
	hue, _ := strconv.ParseFloat(c.PostForm("hue"), 64)
	lightness, _ := strconv.ParseFloat(c.PostForm("lightness"), 64)
	curvesPoints := c.PostForm("curves_points")
	levelsBlack, _ := strconv.ParseFloat(c.PostForm("levels_black"), 64)
	levelsWhite, _ := strconv.ParseFloat(c.PostForm("levels_white"), 64)
	levelsGamma, _ := strconv.ParseFloat(c.PostForm("levels_gamma"), 64)
	selectiveColor := c.PostForm("selective_color")
	chromaticAberration, _ := strconv.ParseFloat(c.PostForm("chromatic_aberration"), 64)
	unsharpAmount, _ := strconv.ParseFloat(c.PostForm("unsharp_amount"), 64)
	unsharpRadius, _ := strconv.ParseFloat(c.PostForm("unsharp_radius"), 64)
	grainAmount, _ := strconv.ParseFloat(c.PostForm("grain_amount"), 64)
	temperature, _ := strconv.ParseFloat(c.PostForm("temperature"), 64)
	tint, _ := strconv.ParseFloat(c.PostForm("tint"), 64)
	shadowRecovery, _ := strconv.ParseFloat(c.PostForm("shadow_recovery"), 64)
	highlightRecovery, _ := strconv.ParseFloat(c.PostForm("highlight_recovery"), 64)
	vignetteAmount, _ := strconv.ParseFloat(c.PostForm("vignette_amount"), 64)
	vignetteFeather, _ := strconv.ParseFloat(c.PostForm("vignette_feather"), 64)
	vignetteRoundness, _ := strconv.ParseFloat(c.PostForm("vignette_roundness"), 64)
	vignetteMidpoint, _ := strconv.ParseFloat(c.PostForm("vignette_midpoint"), 64)

	// Branding & Overlays
	watermarkTemplate := c.PostForm("watermark_template")
	watermarkTile := c.PostForm("watermark_tile") == "on"
	watermarkTileSpacing, _ := strconv.Atoi(c.DefaultPostForm("watermark_tile_spacing", "50"))
	watermarkOpacity, _ := strconv.ParseFloat(c.DefaultPostForm("watermark_opacity", "0"), 64)
	qrCodeText := c.PostForm("qr_code_text")
	qrCodeSize, _ := strconv.Atoi(c.DefaultPostForm("qr_code_size", "128"))
	qrCodePosition := c.PostForm("qr_code_position")
	barcodeText := c.PostForm("barcode_text")
	barcodeType := c.PostForm("barcode_type")
	roundedCorners, _ := strconv.Atoi(c.DefaultPostForm("rounded_corners", "0"))
	dropShadowOffset, _ := strconv.Atoi(c.DefaultPostForm("drop_shadow_offset", "0"))
	dropShadowBlur, _ := strconv.ParseFloat(c.DefaultPostForm("drop_shadow_blur", "5"), 64)
	dropShadowColor := c.PostForm("drop_shadow_color")
	borderWidth, _ := strconv.Atoi(c.DefaultPostForm("border_width", "0"))
	borderColor := c.PostForm("border_color")
	borderStyle := c.PostForm("border_style")
	placeholderWidth, _ := strconv.Atoi(c.DefaultPostForm("placeholder_width", "0"))
	placeholderHeight, _ := strconv.Atoi(c.DefaultPostForm("placeholder_height", "0"))
	placeholderText := c.PostForm("placeholder_text")
	placeholderBgColor := c.PostForm("placeholder_bg_color")
	placeholderTextColor := c.PostForm("placeholder_text_color")
	steganographyText := c.PostForm("steganography_text")
	signaturePosition := c.PostForm("signature_position")
	signatureOpacity, _ := strconv.ParseFloat(c.DefaultPostForm("signature_opacity", "0.8"), 64)
	signatureScale, _ := strconv.ParseFloat(c.DefaultPostForm("signature_scale", "1.0"), 64)

	// Signature file upload (similar to watermark file upload)
	signatureFile, sigErr := c.FormFile("signature")
	signaturePath := ""
	if sigErr == nil {
		if !processor.IsAllowedImageFile(signatureFile.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Signature file type not allowed: %s", signatureFile.Filename),
			})
			return
		}
		safeSigName := filepath.Base(signatureFile.Filename)
		signaturePath = filepath.Join(s.cfg.UploadFolder, fmt.Sprintf("temp_signature_%d_%s", time.Now().UnixMilli(), safeSigName))
		if err := c.SaveUploadedFile(signatureFile, signaturePath); err == nil {
			defer func() { _ = os.Remove(signaturePath) }()
		} else {
			signaturePath = ""
		}
	}

	// Social Media Optimization
	carouselSlice := c.PostForm("carousel_slice") == "on"
	carouselSliceWidth, _ := strconv.Atoi(c.DefaultPostForm("carousel_slice_width", "1080"))
	carouselSliceHeight, _ := strconv.Atoi(c.DefaultPostForm("carousel_slice_height", "1350"))
	safeZoneOverlay := c.PostForm("safe_zone_overlay") == "on"
	safeZonePlatform := c.PostForm("safe_zone_platform")
	maxFileSizeKB, _ := strconv.Atoi(c.DefaultPostForm("max_file_size_kb", "0"))
	stitchImages := c.PostForm("stitch_images") == "on"
	stitchDirection := c.PostForm("stitch_direction")
	faviconGenerate := c.PostForm("favicon_generate") == "on"
	faviconSizes := c.PostForm("favicon_sizes")
	twitchPanelWidth, _ := strconv.Atoi(c.DefaultPostForm("twitch_panel_width", "320"))
	twitchPanelHeight, _ := strconv.Atoi(c.DefaultPostForm("twitch_panel_height", "160"))

	// Format Support & Encoding
	progressiveJPEG := c.PostForm("progressive_jpeg") == "on"
	base64Output := c.PostForm("base64_output") == "on"
	icoSizes := c.PostForm("ico_sizes")
	losslessWebP := c.PostForm("lossless_webp") == "on"
	toneMapHDR := c.PostForm("tone_map_hdr") == "on"
	tiffExtractPages := c.PostForm("tiff_extract_pages") == "on"
	pdfPage, _ := strconv.Atoi(c.DefaultPostForm("pdf_page", "0"))
	svgScale, _ := strconv.ParseFloat(c.DefaultPostForm("svg_scale", "1.0"), 64)

	// Device frame file upload (similar to watermark/signature upload pattern)
	deviceFrameFile, dfErr := c.FormFile("device_frame")
	deviceFramePath := ""
	if dfErr == nil {
		if !processor.IsAllowedImageFile(deviceFrameFile.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Device frame file type not allowed: %s", deviceFrameFile.Filename),
			})
			return
		}
		safeDFName := filepath.Base(deviceFrameFile.Filename)
		deviceFramePath = filepath.Join(s.cfg.UploadFolder, fmt.Sprintf("temp_deviceframe_%d_%s", time.Now().UnixMilli(), safeDFName))
		if err := c.SaveUploadedFile(deviceFrameFile, deviceFramePath); err == nil {
			defer func() { _ = os.Remove(deviceFramePath) }()
		} else {
			deviceFramePath = ""
		}
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

		// Background Removal
		RemoveBackground:    c.PostForm("remove_background") == "on",
		BgRemovalMethod:     c.PostForm("bg_removal_method"),
		BgRemovalTolerance:  bgRemovalTolerance,
		BgRemovalColor:      c.PostForm("bg_removal_color"),
		BgRemovalEdgeSmooth: bgRemovalEdgeSmooth,

		// Professional Adjustments
		Hue:                 hue,
		Lightness:           lightness,
		CurvesPoints:        curvesPoints,
		LevelsBlack:         levelsBlack,
		LevelsWhite:         levelsWhite,
		LevelsGamma:         levelsGamma,
		SelectiveColor:      selectiveColor,
		ChromaticAberration: chromaticAberration,
		UnsharpAmount:       unsharpAmount,
		UnsharpRadius:       unsharpRadius,
		GrainAmount:         grainAmount,
		Temperature:         temperature,
		Tint:                tint,
		ShadowRecovery:      shadowRecovery,
		HighlightRecovery:   highlightRecovery,
		VignetteAmount:      vignetteAmount,
		VignetteFeather:     vignetteFeather,
		VignetteRoundness:   vignetteRoundness,
		VignetteMidpoint:    vignetteMidpoint,

		// Branding & Overlays
		WatermarkTemplate:    watermarkTemplate,
		WatermarkTile:        watermarkTile,
		WatermarkTileSpacing: watermarkTileSpacing,
		WatermarkOpacity:     watermarkOpacity,
		QRCodeText:           qrCodeText,
		QRCodeSize:           qrCodeSize,
		QRCodePosition:       qrCodePosition,
		BarcodeText:          barcodeText,
		BarcodeType:          barcodeType,
		RoundedCorners:       roundedCorners,
		DropShadowOffset:     dropShadowOffset,
		DropShadowBlur:       dropShadowBlur,
		DropShadowColor:      dropShadowColor,
		BorderWidth:          borderWidth,
		BorderColor:          borderColor,
		BorderStyle:          borderStyle,
		PlaceholderWidth:     placeholderWidth,
		PlaceholderHeight:    placeholderHeight,
		PlaceholderText:      placeholderText,
		PlaceholderBgColor:   placeholderBgColor,
		PlaceholderTextColor: placeholderTextColor,
		SteganographyText:    steganographyText,
		SignaturePath:        signaturePath,
		SignaturePosition:    signaturePosition,
		SignatureOpacity:     signatureOpacity,
		SignatureScale:       signatureScale,

		// Social Media Optimization
		CarouselSlice:       carouselSlice,
		CarouselSliceWidth:  carouselSliceWidth,
		CarouselSliceHeight: carouselSliceHeight,
		SafeZoneOverlay:     safeZoneOverlay,
		SafeZonePlatform:    safeZonePlatform,
		MaxFileSizeKB:       maxFileSizeKB,
		DeviceFramePath:     deviceFramePath,
		StitchImages:        stitchImages,
		StitchDirection:     stitchDirection,
		FaviconGenerate:     faviconGenerate,
		FaviconSizes:        faviconSizes,
		TwitchPanelWidth:    twitchPanelWidth,
		TwitchPanelHeight:   twitchPanelHeight,

		// Format Support & Encoding
		ProgressiveJPEG:  progressiveJPEG,
		Base64Output:     base64Output,
		ICOSizes:         icoSizes,
		LosslessWebP:     losslessWebP,
		ToneMapHDR:       toneMapHDR,
		TIFFExtractPages: tiffExtractPages,
		PDFPage:          pdfPage,
		SVGScale:         svgScale,

		// Privacy & Security
		ZeroLogMode:        c.PostForm("zero_log_mode") == "on",
		EncryptOutput:      c.PostForm("encrypt_output") == "on",
		EncryptionPassword: c.PostForm("encryption_password"),
	}

	var results []processor.ProcessResult
	var processedPaths []string
	var errors []string
	var zeroLogResults []processor.ProcessResult // collect zero-log results for direct response

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

		// Zero-log mode: return image data directly, no file on disk
		if opts.ZeroLogMode && res.ImageData != nil {
			zeroLogResults = append(zeroLogResults, *res)
			continue
		}

		results = append(results, *res)
		processedPaths = append(processedPaths, res.NewFilePath)
		// Include extra files (carousel slides, favicon sizes, manifest.json)
		if len(res.ExtraFiles) > 0 {
			processedPaths = append(processedPaths, res.ExtraFiles...)
		}
	}

	// Zero-log mode: return the first processed image directly as binary
	if opts.ZeroLogMode && len(zeroLogResults) > 0 {
		res := zeroLogResults[0]
		c.Header("Content-Type", res.MimeType)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", res.Filename))
		c.Data(http.StatusOK, res.MimeType, res.ImageData)
		return
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

	// Item 33: Base64 output — return base64-encoded image data instead of file paths
	if opts.Base64Output && len(results) > 0 {
		base64Results := make([]gin.H, 0, len(results))
		for _, res := range results {
			// Load the processed image from disk
			img, err := processor.LoadImageForBase64(res.NewFilePath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to load %s for base64: %v", res.ProcessedName, err))
				continue
			}
			format := opts.Format
			if format == "" || format == "pdf" {
				format = "jpg"
			}
			b64, err := processor.EncodeBase64(img, format, opts.Quality)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to base64-encode %s: %v", res.ProcessedName, err))
				continue
			}
			base64Results = append(base64Results, gin.H{
				"filename": res.ProcessedName,
				"base64":   b64,
				"format":   format,
				"width":    res.NewSize,
			})
			// Clean up the file since we're returning base64 instead
			_ = os.Remove(res.NewFilePath)
		}
		response := gin.H{
			"base64_results": base64Results,
		}
		if len(errors) > 0 {
			response["errors"] = errors
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response := gin.H{
		"results": results,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	// Include private link info for encrypted files
	if opts.EncryptOutput && len(results) > 0 {
		privateLinks := make([]gin.H, 0, len(results))
		for _, res := range results {
			pName := res.ProcessedName
			if pName != "" {
				url := middleware.GenerateSignedURL(pName, s.cfg.SigningKey, s.cfg.LinkExpiry)
				privateLinks = append(privateLinks, gin.H{
					"filename":    pName,
					"private_url": url,
				})
			}
		}
		response["private_links"] = privateLinks
	}

	c.JSON(http.StatusOK, response)
}

// handlePrivateDownload serves a file via a signed, time-limited URL (Item 99).
func (s *Server) handlePrivateDownload(c *gin.Context) {
	filename := filepath.Base(c.Param("filename")) // sanitize path component
	expires := c.Query("expires")
	signature := c.Query("sig")

	if !middleware.ValidateSignedURL(filename, expires, signature, s.cfg.SigningKey) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid or expired download link"})
		return
	}

	filePath := filepath.Join(s.cfg.ProcessedFolder, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.FileAttachment(filePath, filename)
}

// handleGenerateLink generates a signed private download link (Item 99).
// Requires API key auth (under /api/v1 group).
func (s *Server) handleGenerateLink(c *gin.Context) {
	filename := c.PostForm("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename is required"})
		return
	}

	url := middleware.GenerateSignedURL(filename, s.cfg.SigningKey, s.cfg.LinkExpiry)
	c.JSON(http.StatusOK, gin.H{
		"url":        url,
		"expires_in": s.cfg.LinkExpiry,
		"expires_at": time.Now().Add(time.Duration(s.cfg.LinkExpiry) * time.Hour).Format(time.RFC3339),
	})
}

// handleDecrypt decrypts an encrypted output file (Item 100).
// Requires API key auth (under /api/v1 group).
func (s *Server) handleDecrypt(c *gin.Context) {
	filename := c.PostForm("filename")
	password := c.PostForm("password")
	if filename == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename and password are required"})
		return
	}

	filePath := filepath.Join(s.cfg.ProcessedFolder, filepath.Base(filename))
	encryptedData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	decryptedData, err := processor.DecryptData(string(encryptedData), password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "decryption failed — wrong password?"})
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", decryptedData)
}
