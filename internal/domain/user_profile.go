package domain

import (
	"strings"
	"time"
)

type UserProfile struct {
	ID           string    `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID       string    `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	DisplayName  *string   `gorm:"column:display_name" json:"display_name"`
	AvatarFileID *string   `gorm:"column:avatar_file_id" json:"avatar_file_id"`
	AvatarURL    *string   `gorm:"-" json:"avatar_url,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (UserProfile) TableName() string {
	return "user_profile"
}

func (p *UserProfile) Update(displayName, avatarFileID *string) {
	if displayName != nil {
		name := strings.TrimSpace(*displayName)
		if name == "" {
			p.DisplayName = nil
		} else {
			p.DisplayName = &name
		}
	}

	if avatarFileID != nil {
		fileID := strings.TrimSpace(*avatarFileID)
		if fileID == "" {
			p.AvatarFileID = nil
			p.AvatarURL = nil
		} else {
			p.AvatarFileID = &fileID
			p.AvatarURL = nil
		}
	}
}

func (p *UserProfile) WithAvatarURL(urlResolver func(string) string) {
	if p == nil || p.AvatarFileID == nil || urlResolver == nil {
		return
	}

	fileID := strings.TrimSpace(*p.AvatarFileID)
	if fileID == "" {
		return
	}

	avatarURL := urlResolver(fileID)
	p.AvatarURL = &avatarURL
}
