package nats

import (
	"context"
	"encoding/json"
	"strings"

	natsgo "github.com/nats-io/nats.go"
	"gorm.io/gorm"

	repo "github.com/example/user-service/internal/adapters/postgres"
	"github.com/example/user-service/internal/domain"
)

type CreateUserHandler struct {
	users    repo.UserRepository
	profiles repo.UserProfileRepository
}

func NewCreateUserHandler(users repo.UserRepository, profiles repo.UserProfileRepository) *CreateUserHandler {
	return &CreateUserHandler{users: users, profiles: profiles}
}

// Handle processes user.create-user requests.
func (h *CreateUserHandler) Handle(msg *natsgo.Msg) {
	var req struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Source string `json:"source"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		Respond(msg, map[string]interface{}{"ok": false, "error": "invalid_payload"})
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		Respond(msg, map[string]interface{}{"ok": false, "error": "id_required"})
		return
	}
	ctx := context.Background()
	if _, err := h.users.FindByID(ctx, req.ID); err == nil {
		Respond(msg, map[string]interface{}{"ok": true})
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		Respond(msg, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	user := &domain.User{ID: req.ID}
	if err := user.SetStatus(domain.UserStatusNew); err != nil {
		Respond(msg, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	user.Email = strings.TrimSpace(req.Email)
	if err := h.users.Create(ctx, user); err != nil {
		Respond(msg, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	profile := &domain.UserProfile{UserID: user.ID}
	_ = h.profiles.Create(ctx, profile)
	Respond(msg, map[string]interface{}{"ok": true})
}
