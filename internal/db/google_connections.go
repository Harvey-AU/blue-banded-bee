package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ErrGoogleConnectionNotFound is returned when a Google Analytics connection is not found
var ErrGoogleConnectionNotFound = errors.New("google analytics connection not found")

// GoogleAnalyticsConnection represents an organisation's connection to a GA4 property
type GoogleAnalyticsConnection struct {
	ID               string
	OrganisationID   string
	GA4PropertyID    string    // GA4 property ID (e.g., "123456789")
	GA4PropertyName  string    // Display name of the property
	GoogleUserID     string    // Google user ID who authorised
	GoogleEmail      string    // Google email for display
	VaultSecretName  string    // Name of the secret in Supabase Vault
	InstallingUserID string    // Our user who installed
	LastSyncedAt     time.Time // When analytics data was last synced
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreateGoogleConnection creates a new Google Analytics connection for an organisation
// Note: Use StoreGoogleToken after creating the connection to store the refresh token in Vault
func (db *DB) CreateGoogleConnection(ctx context.Context, conn *GoogleAnalyticsConnection) error {
	query := `
		INSERT INTO google_analytics_connections (
			id, organisation_id, ga4_property_id, ga4_property_name, google_user_id, google_email,
			installing_user_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organisation_id, ga4_property_id)
		DO UPDATE SET
			ga4_property_name = EXCLUDED.ga4_property_name,
			google_user_id = EXCLUDED.google_user_id,
			google_email = EXCLUDED.google_email,
			installing_user_id = EXCLUDED.installing_user_id,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	err := db.client.QueryRowContext(ctx, query,
		conn.ID, conn.OrganisationID, conn.GA4PropertyID, conn.GA4PropertyName,
		conn.GoogleUserID, conn.GoogleEmail, conn.InstallingUserID,
		conn.CreatedAt, conn.UpdatedAt,
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
		return "", fmt.Errorf("google token not found for connection %s", connectionID)
	}

	return token.String, nil
}

// GetGoogleConnection retrieves a Google Analytics connection by ID
func (db *DB) GetGoogleConnection(ctx context.Context, connectionID string) (*GoogleAnalyticsConnection, error) {
	conn := &GoogleAnalyticsConnection{}
	var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleUserID, googleEmail sql.NullString
	var lastSyncedAt sql.NullTime

	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_user_id, google_email,
		       vault_secret_name, installing_user_id, last_synced_at,
		       created_at, updated_at
		FROM google_analytics_connections
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, connectionID).Scan(
		&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleUserID, &googleEmail,
		&vaultSecretName, &installingUserID, &lastSyncedAt,
		&conn.CreatedAt, &conn.UpdatedAt,
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
	if lastSyncedAt.Valid {
		conn.LastSyncedAt = lastSyncedAt.Time
	}

	return conn, nil
}

// ListGoogleConnections lists all Google Analytics connections for an organisation
func (db *DB) ListGoogleConnections(ctx context.Context, organisationID string) ([]*GoogleAnalyticsConnection, error) {
	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_user_id, google_email,
		       vault_secret_name, installing_user_id, last_synced_at,
		       created_at, updated_at
		FROM google_analytics_connections
		WHERE organisation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.client.QueryContext(ctx, query, organisationID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to list Google Analytics connections")
		return nil, fmt.Errorf("failed to list Google Analytics connections: %w", err)
	}
	defer rows.Close()

	var connections []*GoogleAnalyticsConnection
	for rows.Next() {
		conn := &GoogleAnalyticsConnection{}
		var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleUserID, googleEmail sql.NullString
		var lastSyncedAt sql.NullTime

		err := rows.Scan(
			&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleUserID, &googleEmail,
			&vaultSecretName, &installingUserID, &lastSyncedAt,
			&conn.CreatedAt, &conn.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan Google Analytics connection row")
			return nil, fmt.Errorf("failed to scan Google Analytics connection: %w", err)
		}

		if ga4PropertyID.Valid {
			conn.GA4PropertyID = ga4PropertyID.String
		}
		if ga4PropertyName.Valid {
			conn.GA4PropertyName = ga4PropertyName.String
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
