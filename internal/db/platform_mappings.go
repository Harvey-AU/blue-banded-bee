package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrPlatformOrgMappingNotFound is returned when a platform mapping does not exist.
var ErrPlatformOrgMappingNotFound = errors.New("platform org mapping not found")

// PlatformOrgMapping links an external platform identity to an organisation.
type PlatformOrgMapping struct {
	ID             string
	Platform       string
	PlatformID     string
	PlatformName   *string
	OrganisationID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedBy      *string
}

// UpsertPlatformOrgMapping creates or updates a platform mapping.
func (db *DB) UpsertPlatformOrgMapping(ctx context.Context, mapping *PlatformOrgMapping) error {
	if mapping == nil {
		return fmt.Errorf("mapping is required")
	}

	query := `
		INSERT INTO platform_org_mappings (
			platform, platform_id, platform_name, organisation_id, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (platform, platform_id)
		DO UPDATE SET
			organisation_id = EXCLUDED.organisation_id,
			platform_name = EXCLUDED.platform_name,
			created_by = EXCLUDED.created_by,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	var platformName sql.NullString
	if mapping.PlatformName != nil {
		platformName = sql.NullString{String: *mapping.PlatformName, Valid: true}
	}

	var createdBy sql.NullString
	if mapping.CreatedBy != nil {
		createdBy = sql.NullString{String: *mapping.CreatedBy, Valid: true}
	}

	if err := db.client.QueryRowContext(
		ctx,
		query,
		mapping.Platform,
		mapping.PlatformID,
		platformName,
		mapping.OrganisationID,
		createdBy,
	).Scan(&mapping.ID, &mapping.CreatedAt, &mapping.UpdatedAt); err != nil {
		return fmt.Errorf("failed to upsert platform org mapping: %w", err)
	}

	return nil
}

// GetPlatformOrgMapping returns the mapping for a platform identity.
func (db *DB) GetPlatformOrgMapping(ctx context.Context, platform, platformID string) (*PlatformOrgMapping, error) {
	query := `
		SELECT id, platform, platform_id, platform_name, organisation_id, created_at, updated_at, created_by
		FROM platform_org_mappings
		WHERE platform = $1 AND platform_id = $2
	`

	mapping := &PlatformOrgMapping{}
	var platformName sql.NullString
	var createdBy sql.NullString
	err := db.client.QueryRowContext(ctx, query, platform, platformID).Scan(
		&mapping.ID,
		&mapping.Platform,
		&mapping.PlatformID,
		&platformName,
		&mapping.OrganisationID,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
		&createdBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPlatformOrgMappingNotFound
		}
		return nil, fmt.Errorf("failed to get platform org mapping: %w", err)
	}

	if platformName.Valid {
		mapping.PlatformName = &platformName.String
	}
	if createdBy.Valid {
		mapping.CreatedBy = &createdBy.String
	}

	return mapping, nil
}
