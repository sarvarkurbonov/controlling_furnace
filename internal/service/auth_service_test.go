package service

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"controlling_furnace/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// mockAuthRepo is a lightweight in-test mock for repository.Authorization.
type mockAuthRepo struct {
	CreateFn        func(username, hash string) (int, error)
	GetByUsernameFn func(username string) (*models.User, error)

	createCalls []struct {
		username string
		hash     string
	}
	getCalls []string
}

func (m *mockAuthRepo) Create(username, hash string) (int, error) {
	m.createCalls = append(m.createCalls, struct {
		username string
		hash     string
	}{username: username, hash: hash})
	return m.CreateFn(username, hash)
}

func (m *mockAuthRepo) GetByUsername(username string) (*models.User, error) {
	m.getCalls = append(m.getCalls, username)
	return m.GetByUsernameFn(username)
}

// --- SignUp tests ---

func TestAuthService_SignUp_SuccessHashesPasswordAndCallsRepo(t *testing.T) {
	mock := &mockAuthRepo{
		CreateFn: func(username, hash string) (int, error) {
			return 42, nil
		},
	}
	svc := NewAuthService(mock)

	id, err := svc.SignUp("alice", "s3cr3t")
	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}

	// Ensure Create called exactly once with hashed password (not equal to raw) and valid bcrypt.
	if len(mock.createCalls) != 1 {
		t.Fatalf("expected 1 Create call, got %d", len(mock.createCalls))
	}
	call := mock.createCalls[0]
	if call.username != "alice" {
		t.Errorf("expected username 'alice', got %q", call.username)
	}
	if call.hash == "s3cr3t" {
		t.Errorf("expected hashed password not equal to raw password")
	}
	if err := verifyPassword(call.hash, "s3cr3t"); err != nil {
		t.Errorf("stored hash does not verify with original password: %v", err)
	}
}

func TestAuthService_SignUp_EmptyPassword(t *testing.T) {
	mock := &mockAuthRepo{
		CreateFn: func(username, hash string) (int, error) {
			t.Fatal("Create should not be called for empty password")
			return 0, nil
		},
	}
	svc := NewAuthService(mock)

	_, err := svc.SignUp("bob", "   ")
	if err == nil {
		t.Fatalf("expected error for empty password, got nil")
	}
	if len(mock.createCalls) != 0 {
		t.Fatalf("expected no Create calls, got %d", len(mock.createCalls))
	}
}

func TestAuthService_SignUp_RepoError(t *testing.T) {
	mock := &mockAuthRepo{
		CreateFn: func(username, hash string) (int, error) {
			return 0, errors.New("db down")
		},
	}
	svc := NewAuthService(mock)

	_, err := svc.SignUp("carl", "pass123")
	if err == nil {
		t.Fatalf("expected repo error, got nil")
	}
}

// --- GenerateToken tests ---

func TestAuthService_GenerateToken_Success(t *testing.T) {
	// Prepare a user with a valid bcrypt hash for the provided password.
	hash, err := hashPassword("letmein")
	if err != nil {
		t.Fatalf("hashPassword failed: %v", err)
	}
	user := &models.User{ID: 7, Username: "diana", PasswordHash: hash}

	mock := &mockAuthRepo{
		GetByUsernameFn: func(username string) (*models.User, error) {
			if username != "diana" {
				t.Fatalf("expected username 'diana', got %q", username)
			}
			return user, nil
		},
	}
	svc := NewAuthService(mock)

	token, err := svc.GenerateToken("diana", "letmein")
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	// Validate the token parses and returns the correct user id.
	uid, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if uid != 7 {
		t.Fatalf("expected user id 7 from token, got %d", uid)
	}

	if len(mock.getCalls) != 1 {
		t.Fatalf("expected 1 GetByUsername call, got %d", len(mock.getCalls))
	}
}

func TestAuthService_GenerateToken_UserNotFound(t *testing.T) {
	mock := &mockAuthRepo{
		GetByUsernameFn: func(username string) (*models.User, error) {
			return nil, nil
		},
	}
	svc := NewAuthService(mock)

	_, err := svc.GenerateToken("ghost", "pw")
	if err == nil {
		t.Fatalf("expected ErrUserNotFound, got nil")
	}
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestAuthService_GenerateToken_InvalidPassword(t *testing.T) {
	// Stored hash for different password.
	correctHash, err := hashPassword("correct")
	if err != nil {
		t.Fatalf("hashPassword failed: %v", err)
	}
	mock := &mockAuthRepo{
		GetByUsernameFn: func(username string) (*models.User, error) {
			return &models.User{ID: 1, Username: "eve", PasswordHash: correctHash}, nil
		},
	}
	svc := NewAuthService(mock)

	_, err = svc.GenerateToken("eve", "wrong")
	if err == nil {
		t.Fatalf("expected ErrInvalidPassword, got nil")
	}
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got: %v", err)
	}
}

func TestAuthService_GenerateToken_RepoError(t *testing.T) {
	mock := &mockAuthRepo{
		GetByUsernameFn: func(username string) (*models.User, error) {
			return nil, errors.New("query failed")
		},
	}
	svc := NewAuthService(mock)

	_, err := svc.GenerateToken("john", "pw")
	if err == nil {
		t.Fatalf("expected repo error, got nil")
	}
}

// --- ParseToken tests ---

func TestAuthService_ParseToken_Success(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{})
	token, err := issueToken(99)
	if err != nil {
		t.Fatalf("issueToken failed: %v", err)
	}

	uid, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}
	if uid != 99 {
		t.Fatalf("expected user id 99, got %d", uid)
	}
}

func TestAuthService_ParseToken_Malformed(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{})
	_, err := svc.ParseToken("not-a-jwt")
	if err == nil {
		t.Fatalf("expected error for malformed token")
	}
}

func TestAuthService_ParseToken_InvalidSignature(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{})

	// Create a token signed with a different key.
	now := time.Now()
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID: 5,
	})
	otherKey := []byte("different-key")
	badToken, err := tk.SignedString(otherKey)
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	_, err = svc.ParseToken(badToken)
	if err == nil {
		t.Fatalf("expected signature verification error")
	}
}

func TestAuthService_ParseToken_Expired(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{})

	// Issue an already expired token using same signing key.
	past := time.Now().Add(-2 * time.Hour)
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(past),
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Minute)),
		},
		UserID: 11,
	})
	expiredToken, err := tk.SignedString([]byte(signingKey))
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	_, err = svc.ParseToken(expiredToken)
	if err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestAuthService_ParseToken_UnexpectedAlg(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{})

	now := time.Now()

	// Generate RSA key for RS256 signing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	tk := jwt.NewWithClaims(jwt.SigningMethodRS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserID: 12,
	})

	// Sanity check: ensure the algorithm is RS256 (non-HMAC)
	if tk.Method.Alg() != jwt.SigningMethodRS256.Alg() {
		t.Fatalf("expected RS256 alg, got %s", tk.Method.Alg())
	}

	tokenStr, err := tk.SignedString(privateKey)
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	_, err = svc.ParseToken(tokenStr)
	if err == nil {
		t.Fatalf("expected error due to unexpected signing method")
	}
}
