// auth_repo_test.go
package repository

import (
	cf "controlling_furnace/internal/models"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func newMockRepo(t *testing.T) (*UserRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	repo := NewUserRepository(db)
	cleanup := func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
		_ = db.Close()
	}
	return repo, mock, cleanup
}

func TestUserRepository_Create(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		passwordHash   string
		mockExpect     func(sqlmock.Sqlmock)
		wantID         int
		wantErr        bool
		errContainsStr string
	}{
		{
			name:         "success",
			username:     "alice",
			passwordHash: "h123",
			mockExpect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(insertUserSQL)).
					WithArgs("alice", "h123").
					WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID:  42,
			wantErr: false,
		},
		{
			name:         "exec error",
			username:     "bob",
			passwordHash: "h456",
			mockExpect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(insertUserSQL)).
					WithArgs("bob", "h456").
					WillReturnError(errors.New("db exec failed"))
			},
			wantID:         0,
			wantErr:        true,
			errContainsStr: "insert user",
		},
		{
			name:         "last insert id error",
			username:     "carol",
			passwordHash: "h789",
			mockExpect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(insertUserSQL)).
					WithArgs("carol", "h789").
					WillReturnResult(sqlmock.NewErrorResult(errors.New("no last id")))
			},
			wantID:         0,
			wantErr:        true,
			errContainsStr: "get last insert id",
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, cleanup := newMockRepo(t)
			defer cleanup()

			tt.mockExpect(mock)

			id, err := repo.Create(tt.username, tt.passwordHash)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContainsStr != "" && !contains(err.Error(), tt.errContainsStr) {
					t.Fatalf("expected error to contain %q, got %q", tt.errContainsStr, err.Error())
				}
				if id != 0 {
					t.Fatalf("expected id=0 on error, got %d", id)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Fatalf("unexpected id: want %d, got %d", tt.wantID, id)
			}
		})
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		mockExpect     func(sqlmock.Sqlmock)
		wantUser       *cf.User
		wantErr        bool
		errContainsStr string
	}{
		{
			name:     "found",
			username: "alice",
			mockExpect: func(m sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "username", "password_hash"}).
					AddRow(7, "alice", "h123")
				m.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameSQL)).
					WithArgs("alice").
					WillReturnRows(rows)
			},
			wantUser: &cf.User{
				ID:           7,
				Username:     "alice",
				PasswordHash: "h123",
			},
			wantErr: false,
		},
		{
			name:     "not found (ErrNoRows)",
			username: "missing",
			mockExpect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameSQL)).
					WithArgs("missing").
					WillReturnError(sql.ErrNoRows)
			},
			wantUser: nil,
			wantErr:  false,
		},
		{
			name:     "query error",
			username: "bob",
			mockExpect: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameSQL)).
					WithArgs("bob").
					WillReturnError(errors.New("db query failed"))
			},
			wantUser:       nil,
			wantErr:        true,
			errContainsStr: "select user",
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			repo, mock, cleanup := newMockRepo(t)
			defer cleanup()

			tt.mockExpect(mock)

			u, err := repo.GetByUsername(tt.username)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContainsStr != "" && !contains(err.Error(), tt.errContainsStr) {
					t.Fatalf("expected error to contain %q, got %q", tt.errContainsStr, err.Error())
				}
				if u != nil {
					t.Fatalf("expected user=nil on error, got %+v", u)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantUser == nil {
				if u != nil {
					t.Fatalf("expected nil user, got %+v", u)
				}
				return
			}
			if u == nil {
				t.Fatalf("expected user, got nil")
			}
			if u.ID != tt.wantUser.ID || u.Username != tt.wantUser.Username || u.PasswordHash != tt.wantUser.PasswordHash {
				t.Fatalf("unexpected user: want %+v, got %+v", tt.wantUser, u)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && regexp.MustCompile(regexp.QuoteMeta(substr)).FindStringIndex(s) != nil)
}
