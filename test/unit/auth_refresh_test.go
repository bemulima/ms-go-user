package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/example/user-service/config"
	authmw "github.com/example/user-service/internal/adapter/http/middleware"
	"github.com/example/user-service/internal/usecase"
	pkglog "github.com/example/user-service/pkg/log"
)

func TestAuthFlow_ExpiredAccessTokenAndRefresh(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:            "secret",
		JWTTTLMinutes:        time.Second,
		JWTRefreshTTLMinutes: 5 * time.Second,
	}
	signer, err := service.NewJWTSigner(cfg)
	require.NoError(t, err)

	users := newFakeUserRepo()
	profiles := newFakeProfileRepo()
	providers := newFakeProviderRepo()
	tarantoolClient := &fakeTarantool{email: "user@example.com", password: "password123"}
	auth := service.NewAuthService(cfg, pkglog.New("test"), users, profiles, providers, tarantoolClient, newFakeRBACClient(), fakePublisher{}, signer, fakeAvatarIngestor{})

	user, tokens, err := auth.VerifySignup(context.Background(), "trace-1", "uuid-1", "0000")
	require.NoError(t, err)
	require.NotNil(t, tokens)

	e := echo.New()
	mw := authmw.NewAuthMiddleware(cfg, pkglog.New("test"), nil, users, nil)
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.Handler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+tokens.AccessToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	time.Sleep(1200 * time.Millisecond)

	expiredReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	expiredReq.Header.Set(echo.HeaderAuthorization, "Bearer "+tokens.AccessToken)
	expiredRec := httptest.NewRecorder()
	e.ServeHTTP(expiredRec, expiredReq)
	assert.Equal(t, http.StatusUnauthorized, expiredRec.Code)

	refreshedUser, refreshedTokens, err := auth.RefreshTokens(context.Background(), "trace-1", tokens.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, refreshedTokens)
	assert.Equal(t, user.ID, refreshedUser.ID)

	refreshReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	refreshReq.Header.Set(echo.HeaderAuthorization, "Bearer "+refreshedTokens.AccessToken)
	refreshRec := httptest.NewRecorder()
	e.ServeHTTP(refreshRec, refreshReq)
	assert.Equal(t, http.StatusOK, refreshRec.Code)
}
