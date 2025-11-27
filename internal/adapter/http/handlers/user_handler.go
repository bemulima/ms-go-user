package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/example/user-service/internal/adapter/filestorage"
	"github.com/example/user-service/internal/adapter/imageprocessor"
	"github.com/example/user-service/internal/domain"
	"github.com/example/user-service/internal/usecase"
	res "github.com/example/user-service/pkg/http"
)

type UserHandler struct {
	users        service.UserService
	storage      filestorage.Client
	imageProc    imageprocessor.Client
	avatarPreset string
	avatarKind   string
}

func NewUserHandler(users service.UserService, storage filestorage.Client, imgProc imageprocessor.Client, avatarPreset, avatarKind string) *UserHandler {
	return &UserHandler{users: users, storage: storage, imageProc: imgProc, avatarPreset: avatarPreset, avatarKind: avatarKind}
}

type updateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type changeEmailStartRequest struct {
	NewEmail string `json:"new_email"`
}

type changeEmailVerifyRequest struct {
	UUID string `json:"uuid"`
	Code string `json:"code"`
}

type attachIdentityRequest struct {
	Provider       string  `json:"provider"`
	ProviderUserID string  `json:"provider_user_id"`
	Email          string  `json:"email"`
	DisplayName    *string `json:"display_name"`
	AvatarURL      *string `json:"avatar_url"`
}

func (h *UserHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/me", h.GetMe)
	g.GET("/:id", h.GetByID)
	g.PATCH("/me", h.UpdateProfile)
	g.POST("/me/avatar", h.UploadAvatar)
	g.POST("/me/change-email/start", h.StartChangeEmail)
	g.POST("/me/change-email/verify", h.VerifyChangeEmail)
	g.POST("/me/identities", h.AttachIdentity)
	g.DELETE("/me/identities/:provider/:provider_user_id", h.RemoveIdentity)
}

func (h *UserHandler) GetMe(c echo.Context) error {
	userID := c.Get("user_id").(string)
	user, err := h.users.GetMe(c.Request().Context(), userID)
	if err != nil {
		return res.ErrorJSON(c, http.StatusNotFound, "not_found", "user not found", requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, user)
}

func (h *UserHandler) GetByID(c echo.Context) error {
	userID := c.Param("id")
	requester := c.Get("user_id").(string)
	user, err := h.users.GetByID(c.Request().Context(), requester, userID)
	if err != nil {
		return res.ErrorJSON(c, http.StatusNotFound, "not_found", "user not found", requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(c echo.Context) error {
	req := new(updateProfileRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	userID := c.Get("user_id").(string)
	profile, err := h.users.UpdateProfile(c.Request().Context(), userID, req.DisplayName, req.AvatarURL)
	if err != nil {
		return res.ErrorJSON(c, http.StatusInternalServerError, "update_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, profile)
}

func (h *UserHandler) StartChangeEmail(c echo.Context) error {
	req := new(changeEmailStartRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	userID := c.Get("user_id").(string)
	uuid, err := h.users.StartEmailChange(c.Request().Context(), userID, req.NewEmail)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "change_email_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusAccepted, map[string]string{"uuid": uuid})
}

func (h *UserHandler) VerifyChangeEmail(c echo.Context) error {
	req := new(changeEmailVerifyRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	userID := c.Get("user_id").(string)
	user, err := h.users.VerifyEmailChange(c.Request().Context(), userID, req.UUID, req.Code)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "change_email_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, user)
}

func (h *UserHandler) AttachIdentity(c echo.Context) error {
	req := new(attachIdentityRequest)
	if err := c.Bind(req); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid payload", requestIDFromCtx(c), nil)
	}
	provider := domain.IdentityProvider(strings.ToLower(req.Provider))
	userID := c.Get("user_id").(string)
	identity, profile, err := h.users.AttachIdentity(c.Request().Context(), userID, provider, req.ProviderUserID, req.Email, req.DisplayName, req.AvatarURL)
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "attach_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusCreated, map[string]interface{}{"identity": identity, "profile": profile})
}

func (h *UserHandler) RemoveIdentity(c echo.Context) error {
	provider := domain.IdentityProvider(strings.ToLower(c.Param("provider")))
	providerUserID := c.Param("provider_user_id")
	userID := c.Get("user_id").(string)
	if err := h.users.RemoveIdentity(c.Request().Context(), userID, provider, providerUserID); err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "detach_failed", err.Error(), requestIDFromCtx(c), nil)
	}
	return res.JSON(c, http.StatusOK, map[string]string{"status": "detached"})
}

const maxAvatarSize = 5 * 1024 * 1024

func (h *UserHandler) UploadAvatar(c echo.Context) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "file is required", requestIDFromCtx(c), nil)
	}
	src, err := fileHeader.Open()
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "file open failed", requestIDFromCtx(c), nil)
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxAvatarSize+1))
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "file read failed", requestIDFromCtx(c), nil)
	}
	if len(data) > maxAvatarSize {
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "file too large", requestIDFromCtx(c), nil)
	}

	userID := c.Get("user_id").(string)
	processingMode := strings.ToUpper(strings.TrimSpace(c.FormValue("processing_mode")))
	if processingMode == "" {
		processingMode = "DISABLED"
	}
	switch processingMode {
	case "EAGER", "LAZY", "DISABLED":
	default:
		return res.ErrorJSON(c, http.StatusBadRequest, "bad_request", "invalid processing_mode", requestIDFromCtx(c), nil)
	}

	uploadResp, err := h.storage.Upload(c.Request().Context(), filestorage.UploadRequest{
		OwnerID:        userID,
		FileKind:       h.avatarKind,
		ProcessingMode: processingMode,
		FileName:       fileHeader.Filename,
		ContentType:    fileHeader.Header.Get(echo.HeaderContentType),
		Data:           data,
	})
	if err != nil {
		return res.ErrorJSON(c, http.StatusBadRequest, "upload_failed", err.Error(), requestIDFromCtx(c), nil)
	}

	avatarURL := h.storage.DownloadURL(uploadResp.ID)
	profile, err := h.users.UpdateProfile(c.Request().Context(), userID, nil, &avatarURL)
	if err != nil {
		return res.ErrorJSON(c, http.StatusInternalServerError, "update_failed", err.Error(), requestIDFromCtx(c), nil)
	}

	if processingMode == "EAGER" && h.imageProc != nil {
		if err := h.imageProc.Generate(c.Request().Context(), uploadResp.ID, userID, h.avatarKind, h.avatarPreset, nil); err != nil {
			return res.ErrorJSON(c, http.StatusInternalServerError, "processing_failed", err.Error(), requestIDFromCtx(c), nil)
		}
	}

	signedURL, _ := h.storage.SignedURL(c.Request().Context(), uploadResp.ID, 15)

	response := map[string]interface{}{
		"file_id":         uploadResp.ID,
		"download_url":    avatarURL,
		"signed_url":      signedURL,
		"profile":         profile,
		"processing_mode": processingMode,
	}
	return res.JSON(c, http.StatusCreated, response)
}
