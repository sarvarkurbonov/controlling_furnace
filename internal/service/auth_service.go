package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"controlling_furnace/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenTTL   = time.Hour   // 1 hour
	signingKey = "asd234asd" // TODO: move to config
)

// Domain errors for auth flows.
var (
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidToken    = errors.New("invalid token")
)

// AuthService handles user auth logic
type AuthService struct {
	authRepo repository.Authorization
}

func NewAuthService(repo repository.Authorization) *AuthService {
	return &AuthService{authRepo: repo}
}

// SignUp hashes password and creates a new user
func (s *AuthService) SignUp(username, password string) (int, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("invalid password: %w", err)
	}
	return s.authRepo.Create(username, hash)
}

// Claims defines JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID int `json:"user_id"`
}

// GenerateToken validates credentials and returns JWT
func (s *AuthService) GenerateToken(username, password string) (string, error) {
	u, err := s.authRepo.GetByUsername(username)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", ErrUserNotFound
	}

	if err := verifyPassword(u.PasswordHash, password); err != nil {
		return "", ErrInvalidPassword
	}

	return issueToken(u.ID)
}

// ParseToken parses JWT and returns userID
func (s *AuthService) ParseToken(accessToken string) (int, error) {
	token, err := jwt.ParseWithClaims(accessToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure HMAC signing is used
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(signingKey), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, ErrInvalidToken
	}

	return claims.UserID, nil
}

// helper: hash password safely
func hashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password is empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// helper: verify password against hash
func verifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// helper: issue a signed JWT for a user
func issueToken(userID int) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID: userID,
	})
	return token.SignedString([]byte(signingKey))
}
