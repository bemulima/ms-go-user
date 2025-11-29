package nats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	natsgo "github.com/nats-io/nats.go"

	"github.com/example/user-service/config"
)

// AuthHandler handles user.verifyJWT requests.
type AuthHandler struct {
	secret   []byte
	pubKey   any
	issuer   string
	audience string
	conn     *natsgo.Conn
	rbacSubj string
}

// NewAuthHandler builds NATS auth handler from config.
func NewAuthHandler(cfg *config.Config, conn *natsgo.Conn, rbacSubject string) (*AuthHandler, error) {
	h := &AuthHandler{
		issuer:   cfg.JWTIssuer,
		audience: cfg.JWTAudience,
		conn:     conn,
		rbacSubj: rbacSubject,
	}
	if cfg.JWTSecret != "" {
		h.secret = []byte(cfg.JWTSecret)
		return h, nil
	}
	if cfg.JWTPublicKey != "" {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKey))
		if err != nil {
			return nil, err
		}
		h.pubKey = key
		return h, nil
	}
	return nil, errors.New("jwt secret or public key required")
}

type verifyJWTRequest struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

type verifyJWTResponse struct {
	OK     bool   `json:"ok"`
	UserID string `json:"user_id,omitempty"`
	Role   string `json:"role,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Handle processes NATS message.
func (h *AuthHandler) Handle(msg *natsgo.Msg) {
	var req verifyJWTRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		respond(msg, verifyJWTResponse{OK: false, Error: "invalid payload"})
		return
	}
	userID, role, err := h.parseToken(req.Token)
	if err != nil {
		respond(msg, verifyJWTResponse{OK: false, Error: err.Error()})
		return
	}
	if req.Role != "" {
		if !strings.EqualFold(role, req.Role) {
			respond(msg, verifyJWTResponse{OK: false, Error: "role mismatch"})
			return
		}
		if h.conn != nil && h.rbacSubj != "" {
			ok, err := h.checkRole(userID, req.Role)
			if err != nil {
				respond(msg, verifyJWTResponse{OK: false, Error: err.Error()})
				return
			}
			if !ok {
				respond(msg, verifyJWTResponse{OK: false, Error: "role not allowed"})
				return
			}
			role = strings.ToUpper(req.Role)
		}
	}
	respond(msg, verifyJWTResponse{OK: true, UserID: userID, Role: role})
}

func (h *AuthHandler) parseToken(token string) (string, string, error) {
	parser := jwt.NewParser(jwt.WithAudience(h.audience), jwt.WithIssuer(h.issuer), jwt.WithLeeway(30*time.Second))
	claims := jwt.MapClaims{}
	parsed, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if h.secret != nil {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return h.secret, nil
		}
		return h.pubKey, nil
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return "", "", errors.New("invalid token")
	}
	if cl, ok := parsed.Claims.(jwt.MapClaims); ok {
		sub, _ := cl["sub"].(string)
		role, _ := cl["role"].(string)
		if sub == "" || role == "" {
			return "", "", errors.New("subject or role missing")
		}
		return sub, strings.ToUpper(role), nil
	}
	return "", "", errors.New("invalid claims")
}

type roleCheckRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type roleCheckResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *AuthHandler) checkRole(userID, role string) (bool, error) {
	if h.conn == nil || h.rbacSubj == "" {
		return false, errors.New("rbac nats not configured")
	}
	req := roleCheckRequest{UserID: userID, Role: role}
	data, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := h.conn.RequestWithContext(ctx, h.rbacSubj, data)
	if err != nil {
		return false, err
	}
	var out roleCheckResponse
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return false, err
	}
	if !out.OK {
		if out.Error != "" {
			return false, errors.New(out.Error)
		}
		return false, errors.New("role denied")
	}
	return true, nil
}

func respond(msg *natsgo.Msg, payload verifyJWTResponse) {
	data, _ := json.Marshal(payload)
	_ = msg.Respond(data)
}
