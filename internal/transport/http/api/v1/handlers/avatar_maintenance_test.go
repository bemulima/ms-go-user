package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestEagerAvatarMaintenanceRejectsBeforeDependencies(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	f, err := mw.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("example")); err != nil {
		t.Fatal(err)
	}
	if err = mw.WriteField("processing_mode", "EAGER"); err != nil {
		t.Fatal(err)
	}
	if err = mw.Close(); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/me/avatar", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "test-user")
	// All effectful dependencies are nil: reaching upload or profile code panics.
	h := NewHandler(nil, nil, nil, "avatar", "USER_MEDIA")
	h.SetImageProcessingMaintenance(true)
	if err = h.UploadAvatar(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "image_processing_maintenance") {
		t.Fatalf("unexpected maintenance response: %d", rec.Code)
	}
}
