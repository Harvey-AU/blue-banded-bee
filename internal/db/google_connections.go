package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// ErrGoogleConnectionNotFound is returned when a Google Analytics connection is not found
var ErrGoogleConnectionNotFound = errors.New("google analytics connection not found")

// ErrGoogleTokenNotFound is returned when a Google Analytics token is not found in vault
var ErrGoogleTokenNotFound = errors.New("google analytics token not found")

// GoogleAnalyticsConnection represents an organisation's connection to a GA4 property
type GoogleAnalyticsConnection struct {
	ID               string
	OrganisationID   string
	GA4PropertyID    string        // GA4 property ID (e.g., "123456789")
	GA4PropertyName  string        // Display name of the property
	GoogleAccountID  string        // GA account ID (e.g., "accounts/123456")
	GoogleUserID     string        // Google user ID who authorised
	GoogleEmail      string        // Google email for display
	VaultSecretName  string        // Name of the secret in Supabase Vault
	InstallingUserID string        // Our user who installed
	Status           string        // "active" or "inactive"
	DomainIDs        pq.Int64Array // Array of domain IDs associated with this property
	LastSyncedAt     time.Time     // When analytics data was last synced
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreateGoogleConnection creates a new Google Analytics connection for an organisation
// Note: Use StoreGoogleToken after creating the connection to store the refresh token in Vault
func (db *DB) CreateGoogleConnection(ctx context.Context, conn *GoogleAnalyticsConnection) error {
	// Default status to inactive if not set
	if conn.Status == "" {
		conn.Status = "inactive"
	}

	query := `
		INSERT INTO google_analytics_connections (
			id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
			google_user_id, google_email, installing_user_id, status, domain_ids, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (organisation_id, ga4_property_id)
		DO UPDATE SET
			ga4_property_name = EXCLUDED.ga4_property_name,
			google_account_id = EXCLUDED.google_account_id,
			google_user_id = EXCLUDED.google_user_id,
			google_email = EXCLUDED.google_email,
			installing_user_id = EXCLUDED.installing_user_id,
			status = EXCLUDED.status,
			domain_ids = EXCLUDED.domain_ids,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	err := db.client.QueryRowContext(ctx, query,
		conn.ID, conn.OrganisationID, conn.GA4PropertyID, conn.GA4PropertyName,
		conn.GoogleAccountID, conn.GoogleUserID, conn.GoogleEmail, conn.InstallingUserID,
		conn.Status, pq.Array(conn.DomainIDs), conn.CreatedAt, conn.UpdatedAt,
	).Scan(&conn.ID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", conn.OrganisationID).Str("ga4_property_id", conn.GA4PropertyID).Msg("Failed to create Google Analytics connection")
		return fmt.Errorf("failed to create Google Analytics connection: %w", err)
	}

	return nil
}

// StoreGoogleToken stores a Google Analytics refresh token in Supabase Vault
func (db *DB) StoreGoogleToken(ctx context.Context, connectionID, refreshToken string) error {
	query := `SELECT store_ga_token($1::uuid, $2)`

	// Function returns secret name but we don't need it - just scan to consume the result
	if err := db.client.QueryRowContext(ctx, query, connectionID, refreshToken).Scan(new(string)); err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to store Google token in vault")
		return fmt.Errorf("failed to store Google token: %w", err)
	}

	return nil
}

// GetGoogleToken retrieves a Google Analytics refresh token from Supabase Vault
func (db *DB) GetGoogleToken(ctx context.Context, connectionID string) (string, error) {
	query := `SELECT get_ga_token($1::uuid)`

	var token sql.NullString
	err := db.client.QueryRowContext(ctx, query, connectionID).Scan(&token)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to get Google token from vault")
		return "", fmt.Errorf("failed to get Google token: %w", err)
	}

	if !token.Valid {
		return "", ErrGoogleTokenNotFound
	}

	return token.String, nil
}

// GetGoogleConnection retrieves a Google Analytics connection by ID
func (db *DB) GetGoogleConnection(ctx context.Context, connectionID string) (*GoogleAnalyticsConnection, error) {
	conn := &GoogleAnalyticsConnection{}
	var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleAccountID, googleUserID, googleEmail, status sql.NullString
	var lastSyncedAt sql.NullTime

	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
		       google_user_id, google_email, vault_secret_name, installing_user_id, status,
		       domain_ids, last_synced_at, created_at, updated_at
		FROM google_analytics_connections
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, connectionID).Scan(
		&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleAccountID,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID, &status,
		&conn.DomainIDs, &lastSyncedAt, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoogleConnectionNotFound
		}
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to get Google Analytics connection")
		return nil, fmt.Errorf("failed to get Google Analytics connection: %w", err)
	}

	if ga4PropertyID.Valid {
		conn.GA4PropertyID = ga4PropertyID.String
	}
	if ga4PropertyName.Valid {
		conn.GA4PropertyName = ga4PropertyName.String
	}
	if googleAccountID.Valid {
		conn.GoogleAccountID = googleAccountID.String
	}
	if googleUserID.Valid {
		conn.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		conn.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		conn.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		conn.InstallingUserID = installingUserID.String
	}
	if status.Valid {
		conn.Status = status.String
	}
	if lastSyncedAt.Valid {
		conn.LastSyncedAt = lastSyncedAt.Time
	}

	return conn, nil
}

// ListGoogleConnections lists all Google Analytics connections for an organisation
func (db *DB) ListGoogleConnections(ctx context.Context, organisationID string) ([]*GoogleAnalyticsConnection, error) {
	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
		       google_user_id, google_email, vault_secret_name, installing_user_id, status,
		       domain_ids, last_synced_at, created_at, updated_at
		FROM google_analytics_connections
		WHERE organisation_id = $1
		ORDER BY status DESC, ga4_property_name ASC
	`

	rows, err := db.client.QueryContext(ctx, query, organisationID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to list Google Analytics connections")
		return nil, fmt.Errorf("failed to list Google Analytics connections: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Error().Err(closeErr).Str("organisation_id", organisationID).Msg("Failed to close rows")
		}
	}()

	var connections []*GoogleAnalyticsConnection
	for rows.Next() {
		conn := &GoogleAnalyticsConnection{}
		var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleAccountID, googleUserID, googleEmail, status sql.NullString
		var lastSyncedAt sql.NullTime

		err := rows.Scan(
			&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleAccountID,
			&googleUserID, &googleEmail, &vaultSecretName, &installingUserID, &status,
			&conn.DomainIDs, &lastSyncedAt, &conn.CreatedAt, &conn.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to scan Google Analytics connection row")
			return nil, fmt.Errorf("failed to scan Google Analytics connection: %w", err)
		}

		if ga4PropertyID.Valid {
			conn.GA4PropertyID = ga4PropertyID.String
		}
		if ga4PropertyName.Valid {
			conn.GA4PropertyName = ga4PropertyName.String
		}
		if googleAccountID.Valid {
			conn.GoogleAccountID = googleAccountID.String
		}
		if googleUserID.Valid {
			conn.GoogleUserID = googleUserID.String
		}
		if googleEmail.Valid {
			conn.GoogleEmail = googleEmail.String
		}
		if vaultSecretName.Valid {
			conn.VaultSecretName = vaultSecretName.String
		}
		if installingUserID.Valid {
			conn.InstallingUserID = installingUserID.String
		}
		if status.Valid {
			conn.Status = status.String
		}
		if lastSyncedAt.Valid {
			conn.LastSyncedAt = lastSyncedAt.Time
		}

		connections = append(connections, conn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating Google Analytics connections: %w", err)
	}

	return connections, nil
}

// DeleteGoogleConnection deletes a Google Analytics connection
func (db *DB) DeleteGoogleConnection(ctx context.Context, connectionID, organisationID string) error {
	query := `
		DELETE FROM google_analytics_connections
		WHERE id = $1 AND organisation_id = $2
	`

	result, err := db.client.ExecContext(ctx, query, connectionID, organisationID)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to delete Google Analytics connection")
		return fmt.Errorf("failed to delete Google Analytics connection: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrGoogleConnectionNotFound
	}

	return nil
}

// UpdateGoogleConnectionStatus updates the status of a Google Analytics connection
func (db *DB) UpdateGoogleConnectionStatus(ctx context.Context, connectionID, organisationID, status string) error {
	if status != "active" && status != "inactive" {
		return fmt.Errorf("invalid status: must be 'active' or 'inactive'")
	}

	query := `
		UPDATE google_analytics_connections
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND organisation_id = $3
	`

	result, err := db.client.ExecContext(ctx, query, status, connectionID, organisationID)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Str("status", status).Msg("Failed to update Google Analytics connection status")
		return fmt.Errorf("failed to update Google Analytics connection status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrGoogleConnectionNotFound
	}

	return nil
}

// GetActiveGAConnectionForOrganisation retrieves the active GA4 connection for an organisation
// Returns nil if no active connection (not an error)
// Deprecated: Use GetActiveGAConnectionForDomain for domain-specific lookups
func (db *DB) GetActiveGAConnectionForOrganisation(ctx context.Context, orgID string) (*GoogleAnalyticsConnection, error) {
	conn := &GoogleAnalyticsConnection{}
	var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleAccountID, googleUserID, googleEmail, status sql.NullString
	var lastSyncedAt sql.NullTime

	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
		       google_user_id, google_email, vault_secret_name, installing_user_id, status,
		       domain_ids, last_synced_at, created_at, updated_at
		FROM google_analytics_connections
		WHERE organisation_id = $1 AND status = 'active'
		LIMIT 1
	`

	err := db.client.QueryRowContext(ctx, query, orgID).Scan(
		&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleAccountID,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID, &status,
		&conn.DomainIDs, &lastSyncedAt, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No active connection is not an error - just means no GA4 integration
			log.Debug().Str("organisation_id", orgID).Msg("No active GA4 connection found for organisation")
			return nil, nil
		}
		log.Error().Err(err).Str("organisation_id", orgID).Msg("Failed to get active Google Analytics connection")
		return nil, fmt.Errorf("failed to get active Google Analytics connection: %w", err)
	}

	// Map nullable fields
	if ga4PropertyID.Valid {
		conn.GA4PropertyID = ga4PropertyID.String
	}
	if ga4PropertyName.Valid {
		conn.GA4PropertyName = ga4PropertyName.String
	}
	if googleAccountID.Valid {
		conn.GoogleAccountID = googleAccountID.String
	}
	if googleUserID.Valid {
		conn.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		conn.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		conn.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		conn.InstallingUserID = installingUserID.String
	}
	if status.Valid {
		conn.Status = status.String
	}
	if lastSyncedAt.Valid {
		conn.LastSyncedAt = lastSyncedAt.Time
	}

	return conn, nil
}

// GetActiveGAConnectionForDomain retrieves the active GA4 connection for a specific domain
// Returns nil if no active connection for this domain (not an error)
func (db *DB) GetActiveGAConnectionForDomain(ctx context.Context, organisationID string, domainID int) (*GoogleAnalyticsConnection, error) {
	conn := &GoogleAnalyticsConnection{}
	var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleAccountID, googleUserID, googleEmail, status sql.NullString
	var lastSyncedAt sql.NullTime

	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
		       google_user_id, google_email, vault_secret_name, installing_user_id, status,
		       domain_ids, last_synced_at, created_at, updated_at
		FROM google_analytics_connections
		WHERE organisation_id = $1
		  AND status = 'active'
		  AND $2 = ANY(domain_ids)
		LIMIT 1
	`

	err := db.client.QueryRowContext(ctx, query, organisationID, domainID).Scan(
		&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleAccountID,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID, &status,
		&conn.DomainIDs, &lastSyncedAt, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No active connection for this domain is not an error
			log.Debug().
				Str("organisation_id", organisationID).
				Int("domain_id", domainID).
				Msg("No active GA4 connection found for domain")
			return nil, nil
		}
		log.Error().
			Err(err).
			Str("organisation_id", organisationID).
			Int("domain_id", domainID).
			Msg("Failed to get active Google Analytics connection for domain")
		return nil, fmt.Errorf("failed to get GA connection for domain: %w", err)
	}

	// Map nullable fields
	if ga4PropertyID.Valid {
		conn.GA4PropertyID = ga4PropertyID.String
	}
	if ga4PropertyName.Valid {
		conn.GA4PropertyName = ga4PropertyName.String
	}
	if googleAccountID.Valid {
		conn.GoogleAccountID = googleAccountID.String
	}
	if googleUserID.Valid {
		conn.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		conn.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		conn.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		conn.InstallingUserID = installingUserID.String
	}
	if status.Valid {
		conn.Status = status.String
	}
	if lastSyncedAt.Valid {
		conn.LastSyncedAt = lastSyncedAt.Time
	}

	return conn, nil
}

// UpdateConnectionLastSync updates the last_synced_at timestamp for a connection
func (db *DB) UpdateConnectionLastSync(ctx context.Context, connectionID string) error {
	query := `
		UPDATE google_analytics_connections
		SET last_synced_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	result, err := db.client.ExecContext(ctx, query, connectionID)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to update connection last sync timestamp")
		return fmt.Errorf("failed to update connection last sync: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrGoogleConnectionNotFound
	}

	log.Debug().Str("connection_id", connectionID).Msg("Updated connection last sync timestamp")
	return nil
}

// MarkConnectionInactive sets a connection status to inactive with a reason logged
func (db *DB) MarkConnectionInactive(ctx context.Context, connectionID, reason string) error {
	query := `
		UPDATE google_analytics_connections
		SET status = 'inactive', updated_at = NOW()
		WHERE id = $1
	`

	result, err := db.client.ExecContext(ctx, query, connectionID)
	if err != nil {
		log.Error().
			Err(err).
			Str("connection_id", connectionID).
			Str("reason", reason).
			Msg("Failed to mark connection as inactive")
		return fmt.Errorf("failed to mark connection inactive: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrGoogleConnectionNotFound
	}

	log.Warn().
		Str("connection_id", connectionID).
		Str("reason", reason).
		Msg("Marked Google Analytics connection as inactive")

	return nil
}
