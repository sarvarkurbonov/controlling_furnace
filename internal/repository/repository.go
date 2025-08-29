package repository

import (
	"context"
	"controlling_furnace/internal/models"
	"database/sql"
	"time"
)

type Authorization interface {
	Create(username, hash string) (int, error)
	GetByUsername(username string) (*models.User, error)
}

type StateRepo interface {
	Save(ctx context.Context, s models.FurnaceState) error
	Load(ctx context.Context) (models.FurnaceState, error)
}

type EventRepo interface {
	Append(ctx context.Context, e models.FurnaceEvent) error
	List(ctx context.Context, from, to time.Time, typ string) ([]models.FurnaceEvent, error)
}

type Repository struct {
	StateRepo StateRepo
	EventRepo EventRepo
	Auth      Authorization
}

// Provide indirection for constructor functions to enable test doubles.
// These default to the real constructors and can be overridden in tests.
var (
	newStateRepoFn = NewStateSQLite
	newEventRepoFn = NewEventSQLite
	newAuthRepoFn  = NewUserRepository
)

func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		StateRepo: newStateRepoFn(db),
		EventRepo: newEventRepoFn(db),
		Auth:      newAuthRepoFn(db),
	}
}
