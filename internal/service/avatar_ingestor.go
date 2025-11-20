package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/example/user-service/internal/ports/filestorage"
	pkglog "github.com/example/user-service/pkg/log"
)

type AvatarIngestor interface {
	Ingest(ctx context.Context, traceID, avatarURL string) (string, error)
}

type avatarIngestor struct {
	storage filestorage.Client
	logger  pkglog.Logger
	client  *http.Client
	limit   int64
}

func NewAvatarIngestor(storage filestorage.Client, logger pkglog.Logger) AvatarIngestor {
	return &avatarIngestor{
		storage: storage,
		logger:  logger,
		client:  &http.Client{Timeout: 5 * time.Second},
		limit:   5 * 1024 * 1024,
	}
}

func (a *avatarIngestor) Ingest(ctx context.Context, traceID, avatarURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return "", err
	}

	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return "", fmt.Errorf("avatar fetch failed: status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, a.limit))
	if err != nil {
		return "", err
	}

	fileName := deriveFileName(avatarURL)
	uploadedURL, err := a.storage.Upload(ctx, fileName, res.Header.Get("Content-Type"), body)
	if err != nil {
		return "", err
	}

	a.logger.Info().Str("trace_id", traceID).Str("avatar", avatarURL).Str("stored_url", uploadedURL).Msg("avatar ingested")
	return uploadedURL, nil
}

func deriveFileName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "avatar"
	}
	base := path.Base(parsed.Path)
	if base == "." || base == "/" || strings.TrimSpace(base) == "" {
		return "avatar"
	}
	return base
}
