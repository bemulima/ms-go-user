package port

import (
	"context"

	"github.com/example/user-service/internal/domain"
)

type UserProfileWriter interface {
	Update(ctx context.Context, profile *domain.UserProfile) error
}

type OAuthAvatar struct {
	Data        []byte
	ContentType string
	FileName    string
}

type OAuthAvatarDownloader interface {
	Download(ctx context.Context, provider, rawURL string) (*OAuthAvatar, error)
}

type OAuthAvatarUpload struct {
	OwnerID  string
	FileKind string
	Avatar   *OAuthAvatar
}

type OAuthAvatarStorage interface {
	Store(ctx context.Context, upload OAuthAvatarUpload) (string, error)
}
