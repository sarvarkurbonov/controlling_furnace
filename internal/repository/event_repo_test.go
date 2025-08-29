// go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"controlling_furnace/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
)

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, _ := context.WithTimeout(context.Background(), 3*time.Second)
	return c
}

func TestAppend_Success_WithDefaults(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := NewEventSQLite(db)

	// We don’t know generated id or exact timestamp string, but we can match Exec and argument count.
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO furnace_events (id, occurred_at, type, message, meta)
		VALUES (?, ?, ?, ?, ?)
	`)).
		// accept any args but ensure count is 5; we can also add arg matchers if you want stricter checks
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(),
			"INFO", "hello",
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Append(ctx(t), models.FurnaceEvent{
		// EventID empty -> repo generates
		// OccurredAt zero -> repo sets UTC now
		Type:        "  info ",
		Description: "hello",
		Metadata:    map[string]any{"a": 1},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestAppend_DBError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := NewEventSQLite(db)

	mock.ExpectExec("INSERT INTO furnace_events").
		WillReturnError(errors.New("down"))

	err = repo.Append(ctx(t), models.FurnaceEvent{
		Type:        "info",
		Description: "x",
		Metadata:    map[string]string{"k": "v"},
	})
	if err == nil || !strings.Contains(err.Error(), "down") {
		t.Fatalf("expected error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestList_NoFilters_And_MetadataParsing(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := NewEventSQLite(db)

	// Build rows: occurred_at must be time.Time for Scan
	now := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	js, _ := json.Marshal(map[string]any{"a": "b"})

	rows := sqlmock.NewRows([]string{"id", "occurred_at", "type", "message", "meta"}).
		AddRow("1", now, "INFO", "m1", string(js)).
		AddRow("2", now.Add(time.Hour), "ERROR", "m2", nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, occurred_at, type, message, meta FROM furnace_events ORDER BY occurred_at ASC`)).
		WillReturnRows(rows)

	got, err := repo.List(ctx(t), time.Time{}, time.Time{}, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].EventID != "1" || got[1].EventID != "2" {
		t.Fatalf("unexpected ids: %v, %v", got[0].EventID, got[1].EventID)
	}
	// metadata parsed
	b1, _ := json.Marshal(got[0].Metadata)
	if string(b1) != string(js) {
		t.Fatalf("metadata mismatch: %s vs %s", string(b1), string(js))
	}
	// nil meta stays nil
	if got[1].Metadata != nil {
		t.Fatalf("expected nil meta, got %#v", got[1].Metadata)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestList_WithFilters_OrderAndArgs(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := NewEventSQLite(db)

	from := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	typ := " error " // will be normalized to ERROR

	query := `SELECT id, occurred_at, type, message, meta FROM furnace_events WHERE occurred_at >= ? AND occurred_at <= ? AND type = ? ORDER BY occurred_at ASC`

	rows := sqlmock.NewRows([]string{"id", "occurred_at", "type", "message", "meta"}).
		AddRow("2", from, "ERROR", "b", nil).
		AddRow("3", to, "ERROR", "c", nil)

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(from.UTC(), to.UTC(), "ERROR").
		WillReturnRows(rows)

	got, err := repo.List(ctx(t), from, to, typ)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].EventID != "2" || got[1].EventID != "3" {
		t.Fatalf("unexpected results: %+v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}

func TestList_ScanError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			
		}
	}(db)

	repo := NewEventSQLite(db)

	rows := sqlmock.NewRows([]string{"id", "occurred_at", "type", "message", "meta"}).
		// occurred_at wrong type to force scan error
		AddRow("x", 123, "INFO", "msg", nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, occurred_at, type, message, meta FROM furnace_events ORDER BY occurred_at ASC`)).
		WillReturnRows(rows)

	_, err = repo.List(ctx(t), time.Time{}, time.Time{}, "")
	if err == nil {
		t.Fatalf("expected scan error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}
