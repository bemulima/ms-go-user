package imageprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"
)

type Client interface {
	Generate(ctx context.Context, originalID, ownerID, fileKind, presetGroup string, variants []string) error
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

type generateRequest struct {
	PresetGroup string   `json:"preset_group"`
	Variants    []string `json:"variants,omitempty"`
	Force       bool     `json:"force_regenerate"`
	OwnerID     string   `json:"owner_id"`
	FileKind    string   `json:"file_kind"`
}

func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &httpClient{baseURL: baseURL, client: &http.Client{Timeout: timeout}}
}

func (c *httpClient) Generate(ctx context.Context, originalID, ownerID, fileKind, presetGroup string, variants []string) error {
	if c.baseURL == "" {
		return fmt.Errorf("image processor url is not configured")
	}
	body := generateRequest{
		PresetGroup: presetGroup,
		Variants:    variants,
		Force:       false,
		OwnerID:     ownerID,
		FileKind:    fileKind,
	}
	payload, _ := json.Marshal(body)
	url := c.baseURL + path.Join("/admin/images", originalID, "variants/generate")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("image processor responded %d", res.StatusCode)
	}
	return nil
}
