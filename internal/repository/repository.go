package repository

import (
	"context"
	"controlling_furnace"
	"database/sql"
	"time"
)

type Authorization interface {
	Create(username, hash string) (int, error)
	GetByUsername(username string) (*controlling_furnace.User, error)
}

type StateRepo interface {
	Save(ctx context.Context, s controlling_furnace.FurnaceState) error
	Load(ctx context.Context) (controlling_furnace.FurnaceState, error)
}

type EventRepo interface {
	Append(ctx context.Context, e controlling_furnace.FurnaceEvent) error
	List(ctx context.Context, from, to time.Time, typ string) ([]controlling_furnace.FurnaceEvent, error)
}

type Repository struct {
	StateRepo StateRepo
	EventRepo EventRepo
	Auth      Authorization
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		StateRepo: NewStateSQLite(db),
		EventRepo: NewEventSQLite(db),
		Auth:      NewUserRepository(db),
	}
}
