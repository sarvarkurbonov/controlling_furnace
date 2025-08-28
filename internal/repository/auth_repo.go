package repository

import (
	cf "controlling_furnace"
	"database/sql"
	"errors"
	"fmt"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Ensure implementation of Authorization interface at compile time.
var _ Authorization = (*UserRepository)(nil)

const (
	insertUserSQL           = `INSERT INTO users (username, password_hash) VALUES (?, ?)`
	selectUserByUsernameSQL = `SELECT id, username, password_hash FROM users WHERE username = ?`
)

// Create inserts a new user and returns its ID.
func (r *UserRepository) Create(username, passwordHash string) (int, error) {
	res, err := r.db.Exec(insertUserSQL, username, passwordHash)
	if err != nil {
		return 0, fmt.Errorf("insert user %q: %w", username, err)
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id for user %q: %w", username, err)
	}
	return int(lastID), nil
}

// GetByUsername fetches a user by username. Returns (nil, nil) if not found.
func (r *UserRepository) GetByUsername(username string) (*cf.User, error) {
	var u cf.User
	err := r.db.QueryRow(selectUserByUsernameSQL, username).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select user %q: %w", username, err)
	}
	return &u, nil
}
