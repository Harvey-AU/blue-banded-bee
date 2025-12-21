package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ErrSchedulerNotFound is returned when a scheduler is not found
var ErrSchedulerNotFound = errors.New("scheduler not found")

// Scheduler represents a recurring job schedule
type Scheduler struct {
	ID                    string
	DomainID              int
	OrganisationID        string
	ScheduleIntervalHours int
	NextRunAt             time.Time
	IsEnabled             bool
	Concurrency           int
	FindLinks             bool
	MaxPages              int
	IncludePaths          []string
	ExcludePaths          []string
	RequiredWorkers       int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// CreateScheduler creates a new scheduler
func (db *DB) CreateScheduler(ctx context.Context, scheduler *Scheduler) error {
	query := `
		INSERT INTO schedulers (
			id, domain_id, organisation_id, schedule_interval_hours, next_run_at,
			is_enabled, concurrency, find_links, max_pages, include_paths,
			exclude_paths, required_workers, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := db.client.ExecContext(ctx, query,
		scheduler.ID, scheduler.DomainID, scheduler.OrganisationID,
		scheduler.ScheduleIntervalHours, scheduler.NextRunAt, scheduler.IsEnabled,
		scheduler.Concurrency, scheduler.FindLinks, scheduler.MaxPages,
		Serialise(scheduler.IncludePaths), Serialise(scheduler.ExcludePaths),
		scheduler.RequiredWorkers, scheduler.CreatedAt, scheduler.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	return nil
}

// GetScheduler retrieves a scheduler by ID
func (db *DB) GetScheduler(ctx context.Context, schedulerID string) (*Scheduler, error) {
	scheduler := &Scheduler{}
	var includePaths, excludePaths sql.NullString

	query := `
		SELECT id, domain_id, organisation_id, schedule_interval_hours, next_run_at,
		       is_enabled, concurrency, find_links, max_pages, include_paths,
		       exclude_paths, required_workers, created_at, updated_at
		FROM schedulers
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, schedulerID).Scan(
		&scheduler.ID, &scheduler.DomainID, &scheduler.OrganisationID,
		&scheduler.ScheduleIntervalHours, &scheduler.NextRunAt, &scheduler.IsEnabled,
		&scheduler.Concurrency, &scheduler.FindLinks, &scheduler.MaxPages,
		&includePaths, &excludePaths, &scheduler.RequiredWorkers,
		&scheduler.CreatedAt, &scheduler.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSchedulerNotFound
		}
		return nil, fmt.Errorf("failed to get scheduler: %w", err)
	}

	if includePaths.Valid && includePaths.String != "" {
		if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
			log.Warn().Err(err).Msg("Failed to deserialise include_paths")
		}
	}
	if excludePaths.Valid && excludePaths.String != "" {
		if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
			log.Warn().Err(err).Msg("Failed to deserialise exclude_paths")
		}
	}

	return scheduler, nil
}

// ListSchedulers retrieves all schedulers for an organisation
func (db *DB) ListSchedulers(ctx context.Context, organisationID string) ([]*Scheduler, error) {
	query := `
		SELECT id, domain_id, organisation_id, schedule_interval_hours, next_run_at,
		       is_enabled, concurrency, find_links, max_pages, include_paths,
		       exclude_paths, required_workers, created_at, updated_at
		FROM schedulers
		WHERE organisation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.client.QueryContext(ctx, query, organisationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedulers: %w", err)
	}
	defer rows.Close()

	var schedulers []*Scheduler
	for rows.Next() {
		scheduler := &Scheduler{}
		var includePaths, excludePaths sql.NullString

		err := rows.Scan(
			&scheduler.ID, &scheduler.DomainID, &scheduler.OrganisationID,
			&scheduler.ScheduleIntervalHours, &scheduler.NextRunAt, &scheduler.IsEnabled,
			&scheduler.Concurrency, &scheduler.FindLinks, &scheduler.MaxPages,
			&includePaths, &excludePaths, &scheduler.RequiredWorkers,
			&scheduler.CreatedAt, &scheduler.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scheduler: %w", err)
		}

		if includePaths.Valid && includePaths.String != "" {
			if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
				log.Warn().Err(err).Msg("Failed to deserialise include_paths")
				scheduler.IncludePaths = []string{}
			}
		}
		if excludePaths.Valid && excludePaths.String != "" {
			if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
				log.Warn().Err(err).Msg("Failed to deserialise exclude_paths")
				scheduler.ExcludePaths = []string{}
			}
		}

		schedulers = append(schedulers, scheduler)
	}

	return schedulers, rows.Err()
}

// UpdateScheduler updates a scheduler's configuration
func (db *DB) UpdateScheduler(ctx context.Context, schedulerID string, updates *Scheduler) error {
	query := `
		UPDATE schedulers
		SET schedule_interval_hours = $1,
		    next_run_at = $2,
		    is_enabled = $3,
		    concurrency = $4,
		    find_links = $5,
		    max_pages = $6,
		    include_paths = $7,
		    exclude_paths = $8,
		    required_workers = $9,
		    updated_at = $10
		WHERE id = $11
	`

	result, err := db.client.ExecContext(ctx, query,
		updates.ScheduleIntervalHours, updates.NextRunAt, updates.IsEnabled,
		updates.Concurrency, updates.FindLinks, updates.MaxPages,
		Serialise(updates.IncludePaths), Serialise(updates.ExcludePaths),
		updates.RequiredWorkers, time.Now().UTC(), schedulerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update scheduler: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSchedulerNotFound
	}

	return nil
}

// DeleteScheduler deletes a scheduler
func (db *DB) DeleteScheduler(ctx context.Context, schedulerID string) error {
	query := `DELETE FROM schedulers WHERE id = $1`

	result, err := db.client.ExecContext(ctx, query, schedulerID)
	if err != nil {
		return fmt.Errorf("failed to delete scheduler: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSchedulerNotFound
	}

	return nil
}

// GetSchedulersReadyToRun retrieves schedulers that are ready to run
func (db *DB) GetSchedulersReadyToRun(ctx context.Context, limit int) ([]*Scheduler, error) {
	query := `
		SELECT id, domain_id, organisation_id, schedule_interval_hours, next_run_at,
		       is_enabled, concurrency, find_links, max_pages, include_paths,
		       exclude_paths, required_workers, created_at, updated_at
		FROM schedulers
		WHERE is_enabled = TRUE
		  AND next_run_at <= NOW()
		ORDER BY next_run_at ASC
		LIMIT $1
	`

	rows, err := db.client.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedulers ready to run: %w", err)
	}
	defer rows.Close()

	var schedulers []*Scheduler
	for rows.Next() {
		scheduler := &Scheduler{}
		var includePaths, excludePaths sql.NullString

		err := rows.Scan(
			&scheduler.ID, &scheduler.DomainID, &scheduler.OrganisationID,
			&scheduler.ScheduleIntervalHours, &scheduler.NextRunAt, &scheduler.IsEnabled,
			&scheduler.Concurrency, &scheduler.FindLinks, &scheduler.MaxPages,
			&includePaths, &excludePaths, &scheduler.RequiredWorkers,
			&scheduler.CreatedAt, &scheduler.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scheduler: %w", err)
		}

		if includePaths.Valid && includePaths.String != "" {
			if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
				log.Warn().Err(err).Msg("Failed to deserialise include_paths")
				scheduler.IncludePaths = []string{}
			}
		}
		if excludePaths.Valid && excludePaths.String != "" {
			if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
				log.Warn().Err(err).Msg("Failed to deserialise exclude_paths")
				scheduler.ExcludePaths = []string{}
			}
		}

		schedulers = append(schedulers, scheduler)
	}

	return schedulers, rows.Err()
}

// UpdateSchedulerNextRun updates only the next_run_at timestamp
func (db *DB) UpdateSchedulerNextRun(ctx context.Context, schedulerID string, nextRun time.Time) error {
	query := `
		UPDATE schedulers
		SET next_run_at = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := db.client.ExecContext(ctx, query, nextRun, time.Now().UTC(), schedulerID)
	if err != nil {
		return fmt.Errorf("failed to update scheduler next run: %w", err)
	}

	return nil
}
