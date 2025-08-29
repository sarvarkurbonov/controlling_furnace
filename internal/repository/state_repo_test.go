package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"

	"controlling_furnace/internal/models"
	"controlling_furnace/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStateSQLite_Save_SetsUTCAndMarshalsErrors_WhenTimeZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	// Prepare inputs: zero UpdatedAt should be replaced by time.Now().UTC().
	state := models.FurnaceState{
		Mode:             "auto",
		CurrentTempC:     123.4,
		TargetTempC:      200.0,
		RemainingSeconds: 3600,
		ErrorCodes:       []string{"E1", "E2"},
		IsRunning:        true,
		// UpdatedAt is zero
	}

	// Matchers for arguments.
	isUTCRecent := sqlmockArgumentFunc(func(v driver.Value) bool {
		tm, ok := v.(time.Time)
		if !ok {
			return false
		}
		// must be in UTC and within a reasonable window from "now"
		if tm.Location() != time.UTC {
			return false
		}
		// allow small delta around now (test execution time)
		now := time.Now().UTC()
		if tm.Before(now.Add(-5*time.Second)) || tm.After(now.Add(5*time.Second)) {
			return false
		}
		return true
	})

	// We don't have direct access to the private SQL constant, so match by fragment.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO furnace_state")).
		WithArgs(
			1, // id constant
			state.Mode,
			state.CurrentTempC,
			state.TargetTempC,
			state.RemainingSeconds,
			`["E1","E2"]`, // JSON marshaled errors
			state.IsRunning,
			isUTCRecent, // UpdatedAt written as UTC "now"
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Save(context.Background(), state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestStateSQLite_Save_PreservesGivenTimeButConvertsToUTC(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	locTokyo, _ := time.LoadLocation("Asia/Tokyo")
	original := time.Date(2023, 10, 5, 12, 34, 56, 0, locTokyo) // non-UTC
	expectedUTC := original.UTC()

	state := models.FurnaceState{
		Mode:             "manual",
		CurrentTempC:     12.3,
		TargetTempC:      45.6,
		RemainingSeconds: 42,
		ErrorCodes:       []string{},
		IsRunning:        false,
		UpdatedAt:        original,
	}

	isExactUTC := sqlmockArgumentFunc(func(v driver.Value) bool {
		tm, ok := v.(time.Time)
		if !ok {
			return false
		}
		return tm.Equal(expectedUTC) && tm.Location() == time.UTC
	})

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO furnace_state")).
		WithArgs(
			1,
			state.Mode,
			state.CurrentTempC,
			state.TargetTempC,
			state.RemainingSeconds,
			"[]", // empty slice -> "[]"
			state.IsRunning,
			isExactUTC, // exact UTC-converted input time
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Save(context.Background(), state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestStateSQLite_Save_ExecErrorIsPropagated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	state := models.FurnaceState{
		Mode:             "auto",
		CurrentTempC:     1,
		TargetTempC:      2,
		RemainingSeconds: 3,
		ErrorCodes:       nil, // marshals to "null"
		IsRunning:        true,
		// UpdatedAt is zero; will be set to now
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO furnace_state")).
		WithArgs(
			1,
			state.Mode,
			state.CurrentTempC,
			state.TargetTempC,
			state.RemainingSeconds,
			"null",
			state.IsRunning,
			sqlmock.AnyArg(), // time
		).
		WillReturnError(errors.New("db down"))

	if err := repo.Save(context.Background(), state); err == nil {
		t.Fatalf("Save() expected error, got nil")
	}
}

func TestStateSQLite_Load_NoRowsReturnsZeroValueAndNilError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, mode, temp_c, target_c, remaining_s, errors, running, updated_at")).
		WithArgs(1).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	// zero value expected
	var zero models.FurnaceState
	if !reflect.DeepEqual(got, zero) {
		t.Fatalf("Load() expected zero state, got: %+v", got)
	}
}

func TestStateSQLite_Load_HappyPath_UnmarshalsAndUTC(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	// Prepare row data
	cols := []string{"id", "mode", "temp_c", "target_c", "remaining_s", "errors", "running", "updated_at"}
	locNY, _ := time.LoadLocation("America/New_York")
	nonUTC := time.Date(2024, 2, 1, 8, 30, 0, 0, locNY)

	rows := sqlmock.NewRows(cols).
		AddRow(
			1,
			"auto",
			123.0,
			150.0,
			900,
			`["OVERHEAT","SENSOR_FAIL"]`,
			true,
			nonUTC, // DB gives a non-UTC time; Load should convert to UTC
		)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, mode, temp_c, target_c, remaining_s, errors, running, updated_at")).
		WithArgs(1).
		WillReturnRows(rows)

	got, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if got.ID != 1 ||
		got.Mode != "auto" ||
		got.CurrentTempC != 123.0 ||
		got.TargetTempC != 150.0 ||
		got.RemainingSeconds != 900 ||
		!got.IsRunning {
		t.Fatalf("Load() unexpected fields: %+v", got)
	}

	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("Load() UpdatedAt not UTC: %v (%v)", got.UpdatedAt, got.UpdatedAt.Location())
	}
	if want := []string{"OVERHEAT", "SENSOR_FAIL"}; !equalStringSlices(got.ErrorCodes, want) {
		t.Fatalf("Load() ErrorCodes mismatch: got=%v want=%v", got.ErrorCodes, want)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestStateSQLite_Load_InvalidErrorsJSON_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	repo := repository.NewStateSQLite(db)

	cols := []string{"id", "mode", "temp_c", "target_c", "remaining_s", "errors", "running", "updated_at"}
	rows := sqlmock.NewRows(cols).
		AddRow(
			1,
			"auto",
			10.0,
			20.0,
			30,
			`{not: "an array"}`, // invalid for []string
			false,
			time.Now(),
		)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, mode, temp_c, target_c, remaining_s, errors, running, updated_at")).
		WithArgs(1).
		WillReturnRows(rows)

	_, err = repo.Load(context.Background())
	if err == nil {
		t.Fatalf("Load() expected error due to invalid errors JSON, got nil")
	}
}

// Helpers

type sqlmockArgumentFunc func(v driver.Value) bool

func (f sqlmockArgumentFunc) Match(v driver.Value) bool {
	return f(v)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
