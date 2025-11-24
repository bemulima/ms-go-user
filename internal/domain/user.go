package domain

import (
	"fmt"
	"time"
)

type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusInactive UserStatus = "INACTIVE"
	UserStatusBlocked  UserStatus = "BLOCKED"
)

type User struct {
	ID           string     `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Email        string     `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash *string    `gorm:"column:password_hash" json:"-"`
	Status       UserStatus `gorm:"column:status;type:text;default:ACTIVE" json:"status"`
	IsActive     bool       `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	Profile      *UserProfile
}

func (User) TableName() string {
	return "user"
}

func (u *User) HasPassword() bool {
	return u.PasswordHash != nil && *u.PasswordHash != ""
}

func (u *User) SetPasswordHash(hash string) {
	u.PasswordHash = &hash
}

func (s UserStatus) IsValid() bool {
	return s == UserStatusActive || s == UserStatusInactive || s == UserStatusBlocked
}

func (u *User) SetStatus(status UserStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid user status")
	}
	u.Status = status
	u.IsActive = status == UserStatusActive
	return nil
}

func (u *User) StatusOrDefault() UserStatus {
	if u.Status.IsValid() {
		return u.Status
	}
	if u.IsActive {
		return UserStatusActive
	}
	return UserStatusInactive
}

func (u *User) Activate() {
	_ = u.SetStatus(UserStatusActive)
}

func (u *User) Deactivate() {
	_ = u.SetStatus(UserStatusInactive)
}

func (u *User) Block() {
	_ = u.SetStatus(UserStatusBlocked)
}
