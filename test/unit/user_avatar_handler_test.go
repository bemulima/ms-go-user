package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/example/user-service/internal/adapters/filestorage"
	"github.com/example/user-service/internal/adapters/http/api/v1"
	"github.com/example/user-service/internal/domain"
)

func TestUploadAvatar_Success(t *testing.T) {
	t.Parallel()

	fs := &stubFilestorage{}
	us := &stubUserService{
		setAvatarFileIDFn: func(ctx context.Context, userID, avatarFileID string) (*domain.UserProfile, error) {
			require.Equal(t, "user-1", userID)
			require.Equal(t, "file-123", avatarFileID)
			return &domain.UserProfile{UserID: userID, AvatarFileID: &avatarFileID}, nil
		},
	}
	handler := v1.NewHandler(us, fs, nil, "avatar", "USER_MEDIA")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, _ = part.Write([]byte("img"))
	require.NoError(t, writer.Close())

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users/me/avatar", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user-1")

	require.NoError(t, handler.UploadAvatar(c))
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "file-123", resp.Data["file_id"])
	require.Equal(t, "http://filestorage/files/file-123/download", resp.Data["download_url"])

	require.Equal(t, "USER_MEDIA", fs.uploadReq.FileKind)
	require.Equal(t, "user-1", fs.uploadReq.OwnerID)
}

func TestUploadAvatar_TooLarge(t *testing.T) {
	t.Parallel()

	fs := &stubFilestorage{}
	us := &stubUserService{}
	handler := v1.NewHandler(us, fs, nil, "avatar", "USER_MEDIA")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, _ = part.Write(bytes.Repeat([]byte("a"), 5*1024*1024+1))
	require.NoError(t, writer.Close())

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users/me/avatar", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user-1")

	require.NoError(t, handler.UploadAvatar(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUploadAvatar_EagerTriggersImageProcessor(t *testing.T) {
	t.Parallel()

	fs := &stubFilestorage{}
	proc := &stubImageProc{}
	us := &stubUserService{}
	handler := v1.NewHandler(us, fs, proc, "avatar", "USER_MEDIA")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, _ = part.Write([]byte("img"))
	_ = writer.WriteField("processing_mode", "EAGER")
	require.NoError(t, writer.Close())

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users/me/avatar", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user-1")

	require.NoError(t, handler.UploadAvatar(c))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "file-123", proc.lastOriginal)
	require.Equal(t, "user-1", proc.lastOwner)
	require.Equal(t, "USER_MEDIA", proc.lastKind)
	require.Equal(t, "avatar", proc.lastPreset)
}

type stubFilestorage struct {
	uploadReq filestorage.UploadRequest
}

func (s *stubFilestorage) Upload(ctx context.Context, req filestorage.UploadRequest) (*filestorage.UploadResponse, error) {
	s.uploadReq = req
	return &filestorage.UploadResponse{ID: "file-123"}, nil
}

func (s *stubFilestorage) SignedURL(ctx context.Context, id string, expiresMinutes int64) (string, error) {
	return "http://filestorage/files/" + id + "/signed", nil
}

func (s *stubFilestorage) DownloadURL(id string) string {
	return "http://filestorage/files/" + id + "/download"
}

type stubImageProc struct {
	lastOriginal string
	lastOwner    string
	lastKind     string
	lastPreset   string
}

func (s *stubImageProc) Generate(ctx context.Context, originalID, ownerID, fileKind, presetGroup string, variants []string) error {
	s.lastOriginal = originalID
	s.lastOwner = ownerID
	s.lastKind = fileKind
	s.lastPreset = presetGroup
	return nil
}

type stubUserService struct {
	updateProfileFn   func(ctx context.Context, userID string, displayName *string) (*domain.UserProfile, error)
	setAvatarFileIDFn func(ctx context.Context, userID, avatarFileID string) (*domain.UserProfile, error)
}

func (s *stubUserService) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	return nil, nil
}
func (s *stubUserService) GetByID(ctx context.Context, requesterID, targetID string) (*domain.User, error) {
	return nil, nil
}
func (s *stubUserService) UpdateProfile(ctx context.Context, userID string, displayName *string) (*domain.UserProfile, error) {
	if s.updateProfileFn != nil {
		return s.updateProfileFn(ctx, userID, displayName)
	}
	return &domain.UserProfile{UserID: userID}, nil
}
func (s *stubUserService) SetAvatarFileID(ctx context.Context, userID, avatarFileID string) (*domain.UserProfile, error) {
	if s.setAvatarFileIDFn != nil {
		return s.setAvatarFileIDFn(ctx, userID, avatarFileID)
	}
	return &domain.UserProfile{UserID: userID, AvatarFileID: &avatarFileID}, nil
}
func (s *stubUserService) StartEmailChange(ctx context.Context, userID, newEmail string) (string, error) {
	return "", nil
}
func (s *stubUserService) VerifyEmailChange(ctx context.Context, userID, uuid, code string) (*domain.User, error) {
	return nil, nil
}
func (s *stubUserService) AttachIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID, email string, displayName, avatarURL *string) (*domain.UserIdentity, *domain.UserProfile, error) {
	return nil, nil, nil
}
func (s *stubUserService) RemoveIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID string) error {
	return nil
}
