package server

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"image-resizer/internal/config"
)

func newTestServer(t *testing.T, maxContentLength int64) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	server := NewServer(&config.Config{
		UploadFolder:     filepath.Join(dir, "uploads"),
		ProcessedFolder:  filepath.Join(dir, "processed"),
		MaxContentLength: maxContentLength,
		Port:             "0",
		APIKey:           "test-api-key",
		Env:              "test",
	})
	for _, path := range []string{server.cfg.UploadFolder, server.cfg.ProcessedFolder} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := server.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	return server
}

func TestHealthz(t *testing.T) {
	server := newTestServer(t, 1024)
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestUploadRejectsOversizedBody(t *testing.T) {
	server := newTestServer(t, 64)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bytes.Repeat([]byte("x"), 128)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, req)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body)
	}
}

func TestUploadRejectsTooManyFiles(t *testing.T) {
	server := newTestServer(t, 1<<20)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := range maxUploadFiles + 1 {
		part, err := writer.CreateFormFile("files[]", "image-"+strconv.Itoa(index)+".png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, req)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusRequestEntityTooLarge, response.Body)
	}
}

func TestUploadProcessesImage(t *testing.T) {
	server := newTestServer(t, 1<<20)

	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files[]", "image.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imageBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("operation", "percentage"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("percentage", "50"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("format", "png"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body)
	}
}

func TestCreateArchiveWritesClosedReadableZip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "processed.png")
	if err := os.WriteFile(source, []byte("processed image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dir, "images.zip")
	if err := createArchive(dir, []string{"../processed.png"}, archivePath); err != nil {
		t.Fatalf("createArchive: %v", err)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close archive: %v", closeErr)
		}
	}()
	if len(reader.File) != 1 || reader.File[0].Name != "processed.png" {
		t.Fatalf("unexpected archive entries: %#v", reader.File)
	}

	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	contents, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read archive entry: read=%v close=%v", readErr, closeErr)
	}
	if string(contents) != "processed image bytes" {
		t.Fatalf("archive contents = %q", contents)
	}
}
