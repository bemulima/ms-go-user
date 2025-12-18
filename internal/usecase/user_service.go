package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/example/user-service/internal/adapters/postgres"
	"github.com/example/user-service/internal/domain"
)

var ErrInvalidAvatarURL = errors.New("invalid avatar_url")

type UserService interface {
	GetMe(ctx context.Context, userID string) (*domain.User, error)
	GetByID(ctx context.Context, requesterID, targetID string) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID string, displayName, avatarURL *string) (*domain.UserProfile, error)
	AttachIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID, email string, displayName, avatarURL *string) (*domain.UserIdentity, *domain.UserProfile, error)
	RemoveIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID string) error
}

type userService struct {
	users      repo.UserRepository
	profiles   repo.UserProfileRepository
	identities repo.UserIdentityRepository
}

func NewUserService(users repo.UserRepository, profiles repo.UserProfileRepository, identities repo.UserIdentityRepository) UserService {
	return &userService{users: users, profiles: profiles, identities: identities}
}

func (s *userService) GetMe(ctx context.Context, userID string) (*domain.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *userService) GetByID(ctx context.Context, requesterID, targetID string) (*domain.User, error) {
	return s.users.FindByID(ctx, targetID)
}

func isValidAvatarURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func (s *userService) UpdateProfile(ctx context.Context, userID string, displayName, avatarURL *string) (*domain.UserProfile, error) {
	if avatarURL != nil && !isValidAvatarURL(*avatarURL) {
		return nil, ErrInvalidAvatarURL
	}
	profile, err := s.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Update(displayName, avatarURL)
	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *userService) AttachIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID, email string, displayName, avatarURL *string) (*domain.UserIdentity, *domain.UserProfile, error) {
	if !provider.IsValid() {
		return nil, nil, fmt.Errorf("unsupported provider")
	}
	if avatarURL != nil && !isValidAvatarURL(*avatarURL) {
		return nil, nil, ErrInvalidAvatarURL
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(user.Email, email) {
		return nil, nil, errors.New("email does not match user")
	}
	if existing, err := s.identities.FindByProviderUserID(ctx, provider, providerUserID); err == nil {
		if existing.UserID != userID {
			return nil, nil, errors.New("identity already linked to another user")
		}
		return existing, user.Profile, nil
	}

	identity := &domain.UserIdentity{
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          strings.ToLower(email),
		DisplayName:    displayName,
		AvatarURL:      avatarURL,
	}
	if err := s.identities.Create(ctx, identity); err != nil {
		return nil, nil, err
	}

	profile, err := s.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	profile.Update(displayName, avatarURL)
	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, nil, err
	}

	return identity, profile, nil
}

func (s *userService) RemoveIdentity(ctx context.Context, userID string, provider domain.IdentityProvider, providerUserID string) error {
	if !provider.IsValid() {
		return fmt.Errorf("unsupported provider")
	}
	identity, err := s.identities.FindByProviderUserID(ctx, provider, providerUserID)
	if err != nil {
		return err
	}
	if identity.UserID != userID {
		return errors.New("identity does not belong to user")
	}
	return s.identities.Delete(ctx, identity)
}
