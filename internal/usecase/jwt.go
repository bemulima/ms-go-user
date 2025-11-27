package service

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/example/user-service/config"
)

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type JWTSigner interface {
	SignAccessToken(subject string, claims map[string]interface{}, ttl time.Duration) (string, error)
	SignRefreshToken(subject string, ttl time.Duration) (string, error)
}

type jwtSigner struct {
	cfg       *config.Config
	hmacKey   []byte
	private   *rsa.PrivateKey
	publicKey *rsa.PublicKey
}

func NewJWTSigner(cfg *config.Config) (JWTSigner, error) {
	signer := &jwtSigner{cfg: cfg}
	if cfg.JWTSecret != "" {
		signer.hmacKey = []byte(cfg.JWTSecret)
		return signer, nil
	}
	if cfg.JWTPrivateKey != "" && cfg.JWTPublicKey != "" {
		priv, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.JWTPrivateKey))
		if err != nil {
			return nil, err
		}
		pub, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKey))
		if err != nil {
			return nil, err
		}
		signer.private = priv
		signer.publicKey = pub
		return signer, nil
	}
	return nil, errors.New("jwt secret or key pair must be provided")
}

func (s *jwtSigner) SignAccessToken(subject string, claims map[string]interface{}, ttl time.Duration) (string, error) {
	token := jwt.New(jwt.GetSigningMethod(s.method()))
	now := time.Now().UTC()
	standardClaims := token.Claims.(jwt.MapClaims)
	standardClaims["sub"] = subject
	standardClaims["iss"] = s.cfg.JWTIssuer
	standardClaims["aud"] = s.cfg.JWTAudience
	standardClaims["exp"] = now.Add(ttl).Unix()
	standardClaims["iat"] = now.Unix()
	for k, v := range claims {
		standardClaims[k] = v
	}
	return s.sign(token)
}

func (s *jwtSigner) SignRefreshToken(subject string, ttl time.Duration) (string, error) {
	token := jwt.New(jwt.GetSigningMethod(s.method()))
	now := time.Now().UTC()
	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = subject
	claims["iss"] = s.cfg.JWTIssuer
	claims["aud"] = s.cfg.JWTAudience
	claims["exp"] = now.Add(ttl).Unix()
	claims["iat"] = now.Unix()
	claims["typ"] = "refresh"
	return s.sign(token)
}

func (s *jwtSigner) sign(token *jwt.Token) (string, error) {
	if s.hmacKey != nil {
		return token.SignedString(s.hmacKey)
	}
	return token.SignedString(s.private)
}

func (s *jwtSigner) method() string {
	if s.hmacKey != nil {
		return jwt.SigningMethodHS256.Alg()
	}
	return jwt.SigningMethodRS256.Alg()
}
