package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/example/user-service/internal/service"
	res "github.com/example/user-service/pkg/http"
)

type AuthHandler struct {
	auth service.AuthService
}

func NewAuthHandler(auth service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type signupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signupResponse struct {
	UUID string `json:"uuid"`
}

type signinRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signinResponse struct {
	Tokens *service.Tokens `json:"tokens"`
}

type codeVerificationRequest struct {
	UUID string `json:"uuid"`
	Code string `json:"code"`
}

type oauthCallbackResponse struct {
	Tokens *service.Tokens `json:"tokens"`
}

type oauthCallbackRequest struct {
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

func (h *AuthHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/signup", h.Signup)
	g.POST("/code-verification", h.Verify)
	g.POST("/signin", h.SignIn)
	g.POST("/oauth/:provider/callback", h.OAuthCallback)
}

func (h *AuthHandler) Signup(c echo.Context) error {
	req := new(signupRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	uuid, err := h.auth.StartSignup(c.Request().Context(), requestIDFromCtx(c), req.Email, req.Password)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "signup_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusAccepted, signupResponse{UUID: uuid})
}

func (h *AuthHandler) Verify(c echo.Context) error {
	req := new(codeVerificationRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	user, tokens, err := h.auth.VerifySignup(c.Request().Context(), requestIDFromCtx(c), req.UUID, req.Code)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "verification_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"user": user, "tokens": tokens})
}

func (h *AuthHandler) SignIn(c echo.Context) error {
	req := new(signinRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	user, tokens, err := h.auth.SignIn(c.Request().Context(), requestIDFromCtx(c), req.Email, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		return res.ErrorJSON(c, status, "signin_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"user": user, "tokens": tokens})
}

func (h *AuthHandler) OAuthCallback(c echo.Context) error {
	provider := c.Param("provider")
	req := new(oauthCallbackRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	if req.Email == "" {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "email required", requestIDFromCtx(c), nil)
	}

	user, tokens, err := h.auth.HandleOAuthCallback(c.Request().Context(), requestIDFromCtx(c), provider, service.OAuthUserInfo{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "oauth_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"user": user, "tokens": tokens})
}

func requestIDFromCtx(c echo.Context) string {
	if reqID := c.Response().Header().Get(echo.HeaderXRequestID); reqID != "" {
		return reqID
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}
