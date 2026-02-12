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
var ErrSchedulerStateConflict = errors.New("scheduler state conflict")

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
		log.Error().Err(err).Str("scheduler_id", scheduler.ID).Str("organisation_id", scheduler.OrganisationID).Msg("Failed to create scheduler")
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
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to get scheduler")
		return nil, fmt.Errorf("failed to get scheduler: %w", err)
	}

	if includePaths.Valid && includePaths.String != "" {
		if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
			log.Warn().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to deserialise include_paths")
			scheduler.IncludePaths = []string{}
		}
	} else {
		scheduler.IncludePaths = []string{}
	}
	if excludePaths.Valid && excludePaths.String != "" {
		if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
			log.Warn().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to deserialise exclude_paths")
			scheduler.ExcludePaths = []string{}
		}
	} else {
		scheduler.ExcludePaths = []string{}
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
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to query schedulers")
		return nil, fmt.Errorf("failed to list schedulers: %w", err)
	}
	defer rows.Close()

	// Initialize slice to return empty array instead of null in JSON
	schedulers := make([]*Scheduler, 0)
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
			log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to scan scheduler row")
			return nil, fmt.Errorf("failed to scan scheduler: %w", err)
		}

		if includePaths.Valid && includePaths.String != "" {
			if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
				log.Warn().Err(err).Str("scheduler_id", scheduler.ID).Msg("Failed to deserialise include_paths")
				scheduler.IncludePaths = []string{}
			}
		} else {
			scheduler.IncludePaths = []string{}
		}
		if excludePaths.Valid && excludePaths.String != "" {
			if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
				log.Warn().Err(err).Str("scheduler_id", scheduler.ID).Msg("Failed to deserialise exclude_paths")
				scheduler.ExcludePaths = []string{}
			}
		} else {
			scheduler.ExcludePaths = []string{}
		}

		schedulers = append(schedulers, scheduler)
	}

	return schedulers, rows.Err()
}

// UpdateScheduler updates a scheduler's configuration.
// If expectedIsEnabled is non-nil, the update is conditional on current is_enabled
// matching the expected value (optimistic concurrency).
func (db *DB) UpdateScheduler(ctx context.Context, schedulerID string, updates *Scheduler, expectedIsEnabled *bool) error {
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

	var result sql.Result
	var err error
	if expectedIsEnabled != nil {
		query = query + " AND is_enabled = $12"
		result, err = db.client.ExecContext(ctx, query,
			updates.ScheduleIntervalHours, updates.NextRunAt, updates.IsEnabled,
			updates.Concurrency, updates.FindLinks, updates.MaxPages,
			Serialise(updates.IncludePaths), Serialise(updates.ExcludePaths),
			updates.RequiredWorkers, time.Now().UTC(), schedulerID, *expectedIsEnabled,
		)
	} else {
		result, err = db.client.ExecContext(ctx, query,
			updates.ScheduleIntervalHours, updates.NextRunAt, updates.IsEnabled,
			updates.Concurrency, updates.FindLinks, updates.MaxPages,
			Serialise(updates.IncludePaths), Serialise(updates.ExcludePaths),
			updates.RequiredWorkers, time.Now().UTC(), schedulerID,
		)
	}
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to update scheduler")
		return fmt.Errorf("failed to update scheduler: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to get rows affected after update")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		if expectedIsEnabled != nil {
			var exists bool
			err := db.client.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schedulers WHERE id = $1)", schedulerID).Scan(&exists)
			if err != nil {
				log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to check scheduler existence after conflict")
				return fmt.Errorf("failed to check scheduler existence: %w", err)
			}
			if exists {
				log.Warn().Str("scheduler_id", schedulerID).Msg("Scheduler state conflict on update")
				return ErrSchedulerStateConflict
			}
		}
		log.Warn().Str("scheduler_id", schedulerID).Msg("Scheduler not found for update")
		return ErrSchedulerNotFound
	}

	return nil
}

// DeleteScheduler deletes a scheduler
func (db *DB) DeleteScheduler(ctx context.Context, schedulerID string) error {
	query := `DELETE FROM schedulers WHERE id = $1`

	result, err := db.client.ExecContext(ctx, query, schedulerID)
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to delete scheduler")
		return fmt.Errorf("failed to delete scheduler: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to get rows affected after delete")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		log.Warn().Str("scheduler_id", schedulerID).Msg("Scheduler not found for deletion")
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
		log.Error().Err(err).Int("limit", limit).Msg("Failed to query schedulers ready to run")
		return nil, fmt.Errorf("failed to get schedulers ready to run: %w", err)
	}
	defer rows.Close()

	// Initialize slice to return empty array instead of null in JSON
	schedulers := make([]*Scheduler, 0)
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
			log.Error().Err(err).Msg("Failed to scan scheduler row in ready to run query")
			return nil, fmt.Errorf("failed to scan scheduler: %w", err)
		}

		if includePaths.Valid && includePaths.String != "" {
			if err := json.Unmarshal([]byte(includePaths.String), &scheduler.IncludePaths); err != nil {
				log.Warn().Err(err).Str("scheduler_id", scheduler.ID).Msg("Failed to deserialise include_paths")
				scheduler.IncludePaths = []string{}
			}
		} else {
			scheduler.IncludePaths = []string{}
		}
		if excludePaths.Valid && excludePaths.String != "" {
			if err := json.Unmarshal([]byte(excludePaths.String), &scheduler.ExcludePaths); err != nil {
				log.Warn().Err(err).Str("scheduler_id", scheduler.ID).Msg("Failed to deserialise exclude_paths")
				scheduler.ExcludePaths = []string{}
			}
		} else {
			scheduler.ExcludePaths = []string{}
		}

		schedulers = append(schedulers, scheduler)
	}

	return schedulers, rows.Err()
}

// GetLastJobStartTimeForScheduler retrieves the most recent started_at time for jobs created by a scheduler
func (db *DB) GetLastJobStartTimeForScheduler(ctx context.Context, schedulerID string) (*time.Time, error) {
	var startedAt sql.NullTime

	query := `
		SELECT started_at
		FROM jobs
		WHERE scheduler_id = $1
		  AND started_at IS NOT NULL
		ORDER BY started_at DESC
		LIMIT 1
	`

	err := db.client.QueryRowContext(ctx, query, schedulerID).Scan(&startedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// No jobs found for this scheduler yet
			return nil, nil
		}
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to get last job start time")
		return nil, fmt.Errorf("failed to get last job start time: %w", err)
	}

	if !startedAt.Valid {
		return nil, nil
	}

	return &startedAt.Time, nil
}

// UpdateSchedulerNextRun updates only the next_run_at timestamp
func (db *DB) UpdateSchedulerNextRun(ctx context.Context, schedulerID string, nextRun time.Time) error {
	query := `
		UPDATE schedulers
		SET next_run_at = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := db.client.ExecContext(ctx, query, nextRun, time.Now().UTC(), schedulerID)
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Time("next_run", nextRun).Msg("Failed to update scheduler next run")
		return fmt.Errorf("failed to update scheduler next run: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Error().Err(err).Str("scheduler_id", schedulerID).Msg("Failed to get rows affected after next run update")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		log.Warn().Str("scheduler_id", schedulerID).Msg("Scheduler not found for next run update")
		return ErrSchedulerNotFound
	}

	return nil
}
