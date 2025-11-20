package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path"
	"time"
)

type Client interface {
	Upload(ctx context.Context, fileName, contentType string, data []byte) (string, error)
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

type uploadResponse struct {
	URL string `json:"url"`
}

func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &httpClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *httpClient) Upload(ctx context.Context, fileName, contentType string, data []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", path.Base(fileName))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if contentType != "" {
		_ = writer.WriteField("content_type", contentType)
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/files", c.baseURL), body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return "", fmt.Errorf("filestorage error: status %d", res.StatusCode)
	}

	var resp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", err
	}

	if resp.URL == "" {
		return "", fmt.Errorf("filestorage response missing url")
	}

	return resp.URL, nil
}
