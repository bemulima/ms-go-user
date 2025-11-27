package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"time"
)

type Client interface {
	Upload(ctx context.Context, req UploadRequest) (*UploadResponse, error)
	SignedURL(ctx context.Context, id string, expiresMinutes int64) (string, error)
	DownloadURL(id string) string
}

type UploadRequest struct {
	OwnerID        string
	FileKind       string
	ProcessingMode string
	FileName       string
	ContentType    string
	Data           []byte
}

type UploadResponse struct {
	ID string `json:"id"`
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

type uploadResponse struct {
	ID string `json:"id"`
}

type signedURLResponse struct {
	URL string `json:"url"`
}

func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &httpClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *httpClient) Upload(ctx context.Context, req UploadRequest) (*UploadResponse, error) {
	if req.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if req.FileKind == "" {
		return nil, fmt.Errorf("file_kind is required")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", path.Base(req.FileName))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(req.Data); err != nil {
		return nil, err
	}
	if req.ContentType != "" {
		_ = writer.WriteField("content_type", req.ContentType)
	}
	_ = writer.WriteField("owner_id", req.OwnerID)
	_ = writer.WriteField("file_kind", req.FileKind)
	if req.ProcessingMode != "" {
		_ = writer.WriteField("image_processing_mode", req.ProcessingMode)
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/files/upload", c.baseURL), body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("filestorage error: status %d: %s", res.StatusCode, string(data))
	}

	var resp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}

	if resp.ID == "" {
		return nil, fmt.Errorf("filestorage response missing id")
	}

	return &UploadResponse{ID: resp.ID}, nil
}

func (c *httpClient) SignedURL(ctx context.Context, id string, expiresMinutes int64) (string, error) {
	if expiresMinutes <= 0 {
		expiresMinutes = 15
	}
	body := map[string]any{
		"purpose":         "download",
		"method":          "GET",
		"expires_minutes": expiresMinutes,
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/files/%s/signed-url", c.baseURL, id), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		data, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("filestorage error: status %d: %s", res.StatusCode, string(data))
	}
	var resp signedURLResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", err
	}
	if resp.URL == "" {
		return "", fmt.Errorf("filestorage response missing url")
	}
	return resp.URL, nil
}

func (c *httpClient) DownloadURL(id string) string {
	return fmt.Sprintf("%s/files/%s/download", c.baseURL, id)
}
