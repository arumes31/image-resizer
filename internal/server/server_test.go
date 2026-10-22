package server

import (
	"bytes"
	"image"
	"image-resizer/internal/config"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	cfg := &config.Config{
		UploadFolder:       filepath.Join(dir, "uploads"),
		ProcessedFolder:    filepath.Join(dir, "processed"),
		MaxContentLength:   1024 * 1024,
		MaxFilesPerRequest: 2,
		Port:               "0",
	}
	for _, d := range []string{cfg.UploadFolder, cfg.ProcessedFolder} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{router: gin.New(), cfg: cfg, jobs: make(chan struct{}, 1)}
	s.setupRoutes()
	return s
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadRequest(t *testing.T, fields map[string]string, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range files {
		fw, err := mw.CreateFormFile("files[]", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadRejectsTraversalTemplate(t *testing.T) {
	s := newTestServer(t)
	req := uploadRequest(t,
		map[string]string{"format": "png", "rename_template": "../../../../tmp/pwn"},
		map[string][]byte{"a.png": pngBytes(t)})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadRejectsUnknownFormat(t *testing.T) {
	s := newTestServer(t)
	req := uploadRequest(t, map[string]string{"format": "html"}, map[string][]byte{"a.png": pngBytes(t)})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUploadBodyLimit(t *testing.T) {
	s := newTestServer(t)
	big := make([]byte, 2*1024*1024)
	req := uploadRequest(t, nil, map[string][]byte{"a.png": big})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadFileCountLimit(t *testing.T) {
	s := newTestServer(t)
	img := pngBytes(t)
	req := uploadRequest(t, nil, map[string][]byte{"a.png": img, "b.png": img, "c.png": img})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUploadSuccessHidesServerPaths(t *testing.T) {
	s := newTestServer(t)
	req := uploadRequest(t, map[string]string{"format": "png"}, map[string][]byte{"IMG_0001.png": pngBytes(t)})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "newFilePath") || strings.Contains(body, s.cfg.ProcessedFolder) {
		t.Fatalf("response leaks server path: %s", body)
	}
	if !strings.Contains(body, "processed_IMG_0001_") {
		t.Fatalf("unexpected response: %s", body)
	}
	// Uploads must not be left behind.
	entries, err := os.ReadDir(s.cfg.UploadFolder)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("upload folder not cleaned up: %d entries", len(entries))
	}
}

func TestUploadInvalidImageGenericError(t *testing.T) {
	s := newTestServer(t)
	req := uploadRequest(t, nil, map[string][]byte{"a.png": []byte("not an image")})
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), s.cfg.UploadFolder) {
		t.Fatalf("error leaks server path: %s", w.Body.String())
	}
}
