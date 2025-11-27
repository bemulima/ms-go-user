package rbac

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type Client interface {
	GetRoleByUserID(ctx context.Context, userID string) (string, error)
	GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error)
	CheckPermission(ctx context.Context, userID, permission string) (bool, error)
	CheckRole(ctx context.Context, userID, role string) (bool, error)
	AssignRole(ctx context.Context, userID, role string) error
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

type cachingClient struct {
	delegate Client
	ttl      time.Duration
	cache    map[string]cacheEntry
	mu       sync.RWMutex
}

func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &httpClient{baseURL: baseURL, client: &http.Client{Timeout: timeout}}
}

func NewCachingClient(delegate Client, ttl time.Duration) Client {
	return &cachingClient{delegate: delegate, ttl: ttl, cache: map[string]cacheEntry{}}
}

func (c *httpClient) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	var resp struct {
		Role string `json:"role"`
	}
	if err := c.get(ctx, "/get_role_by_user_id", url.Values{"user_id": {userID}}, &resp); err != nil {
		return "", err
	}
	return resp.Role, nil
}

func (c *httpClient) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	var resp struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.get(ctx, "/get_permissions_by_user_id_for_role", url.Values{"user_id": {userID}}, &resp); err != nil {
		return nil, err
	}
	return resp.Permissions, nil
}

func (c *httpClient) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	var resp struct {
		Allowed bool `json:"allowed"`
	}
	if err := c.get(ctx, "/check_permission_by_user_id", url.Values{"user_id": {userID}, "permission": {permission}}, &resp); err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

func (c *httpClient) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	var resp struct {
		Allowed bool `json:"allowed"`
	}
	if err := c.get(ctx, "/check_role_by_user_id", url.Values{"user_id": {userID}, "role": {role}}, &resp); err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

func (c *httpClient) AssignRole(ctx context.Context, userID, role string) error {
	payload := map[string]interface{}{
		"value": map[string]string{
			"user_id": userID,
			"role":    role,
		},
	}
	return c.post(ctx, "/assign_role", payload)
}

func (c *httpClient) get(ctx context.Context, path string, params url.Values, out interface{}) error {
	op := func() error {
		endpoint := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return backoff.Permanent(err)
		}
		res, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode >= 400 {
			return fmt.Errorf("rbac error: status %d", res.StatusCode)
		}
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return backoff.Permanent(err)
		}
		return nil
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	bo.MaxElapsedTime = 3 * time.Second

	return backoff.Retry(op, backoff.WithContext(bo, ctx))
}

func (c *httpClient) post(ctx context.Context, path string, payload interface{}) error {
	op := func() error {
		body, err := json.Marshal(payload)
		if err != nil {
			return backoff.Permanent(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s%s", c.baseURL, path), bytes.NewReader(body))
		if err != nil {
			return backoff.Permanent(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode >= 400 {
			return fmt.Errorf("rbac error: status %d", res.StatusCode)
		}
		return nil
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	bo.MaxElapsedTime = 3 * time.Second

	return backoff.Retry(op, backoff.WithContext(bo, ctx))
}

func (c *cachingClient) cacheKey(parts ...string) string {
	return fmt.Sprintf("%v", parts)
}

func (c *cachingClient) withCache(key string, loader func() (interface{}, error)) (interface{}, error) {
	now := time.Now()
	c.mu.RLock()
	if entry, ok := c.cache[key]; ok && now.Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.value, nil
	}
	c.mu.RUnlock()

	value, err := loader()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return value, nil
}

func (c *cachingClient) GetRoleByUserID(ctx context.Context, userID string) (string, error) {
	key := c.cacheKey("role", userID)
	value, err := c.withCache(key, func() (interface{}, error) {
		return c.delegate.GetRoleByUserID(ctx, userID)
	})
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func (c *cachingClient) GetPermissionsByUserID(ctx context.Context, userID string) ([]string, error) {
	key := c.cacheKey("perms", userID)
	value, err := c.withCache(key, func() (interface{}, error) {
		return c.delegate.GetPermissionsByUserID(ctx, userID)
	})
	if err != nil {
		return nil, err
	}
	return value.([]string), nil
}

func (c *cachingClient) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	key := c.cacheKey("perm", userID, permission)
	value, err := c.withCache(key, func() (interface{}, error) {
		return c.delegate.CheckPermission(ctx, userID, permission)
	})
	if err != nil {
		return false, err
	}
	return value.(bool), nil
}

func (c *cachingClient) CheckRole(ctx context.Context, userID, role string) (bool, error) {
	key := c.cacheKey("role-check", userID, role)
	value, err := c.withCache(key, func() (interface{}, error) {
		return c.delegate.CheckRole(ctx, userID, role)
	})
	if err != nil {
		return false, err
	}
	return value.(bool), nil
}

func (c *cachingClient) AssignRole(ctx context.Context, userID, role string) error {
	return c.delegate.AssignRole(ctx, userID, role)
}
