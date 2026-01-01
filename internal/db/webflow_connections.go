package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ErrWebflowConnectionNotFound is returned when a webflow connection is not found
var ErrWebflowConnectionNotFound = errors.New("webflow connection not found")

// WebflowConnection represents an organisation's connection to a Webflow Workspace/User
// Note: While OAuth is user-workspace scoped, we map it to an Organisation in our system.
type WebflowConnection struct {
	ID                 string
	OrganisationID     string
	WebflowWorkspaceID string
	AuthedUserID       string
	VaultSecretName    string // Name of the secret in Supabase Vault
	InstallingUserID   string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateWebflowConnection creates a new Webflow connection for an organisation
// Note: Use StoreWebflowToken after creating the connection to store the access token in Vault
func (db *DB) CreateWebflowConnection(ctx context.Context, conn *WebflowConnection) error {
	query := `
		INSERT INTO webflow_connections (
			id, organisation_id, webflow_workspace_id, authed_user_id,
			installing_user_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (organisation_id, authed_user_id)
		DO UPDATE SET
			webflow_workspace_id = EXCLUDED.webflow_workspace_id,
			installing_user_id = EXCLUDED.installing_user_id,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	err := db.client.QueryRowContext(ctx, query,
		conn.ID, conn.OrganisationID, conn.WebflowWorkspaceID, conn.AuthedUserID,
		conn.InstallingUserID, conn.CreatedAt, conn.UpdatedAt,
	).Scan(&conn.ID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", conn.OrganisationID).Str("authed_user_id", conn.AuthedUserID).Msg("Failed to create webflow connection")
		return fmt.Errorf("failed to create webflow connection: %w", err)
	}

	return nil
}

// StoreWebflowToken stores a Webflow access token in Supabase Vault
func (db *DB) StoreWebflowToken(ctx context.Context, connectionID, token string) error {
	query := `SELECT store_webflow_token($1::uuid, $2)`

	// Function returns secret name but we don't need it - just scan to consume the result
	if err := db.client.QueryRowContext(ctx, query, connectionID, token).Scan(new(string)); err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to store webflow token in vault")
		return fmt.Errorf("failed to store webflow token: %w", err)
	}

	return nil
}

// GetWebflowToken retrieves a Webflow access token from Supabase Vault
func (db *DB) GetWebflowToken(ctx context.Context, connectionID string) (string, error) {
	query := `SELECT get_webflow_token($1::uuid)`

	var token sql.NullString
	err := db.client.QueryRowContext(ctx, query, connectionID).Scan(&token)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to get webflow token from vault")
		return "", fmt.Errorf("failed to get webflow token: %w", err)
	}

	if !token.Valid {
		return "", fmt.Errorf("webflow token not found for connection %s", connectionID)
	}

	return token.String, nil
}

// GetWebflowConnection retrieves a Webflow connection by ID
func (db *DB) GetWebflowConnection(ctx context.Context, connectionID string) (*WebflowConnection, error) {
	conn := &WebflowConnection{}
	var installingUserID, vaultSecretName, webflowWorkspaceID, authedUserID sql.NullString

	query := `
		SELECT id, organisation_id, webflow_workspace_id, authed_user_id,
		       vault_secret_name, installing_user_id,
		       created_at, updated_at
		FROM webflow_connections
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, connectionID).Scan(
		&conn.ID, &conn.OrganisationID, &webflowWorkspaceID, &authedUserID,
		&vaultSecretName, &installingUserID,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrWebflowConnectionNotFound
		}
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to get webflow connection")
		return nil, fmt.Errorf("failed to get webflow connection: %w", err)
	}

	if webflowWorkspaceID.Valid {
		conn.WebflowWorkspaceID = webflowWorkspaceID.String
	}
	if authedUserID.Valid {
		conn.AuthedUserID = authedUserID.String
	}
	if vaultSecretName.Valid {
		conn.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		conn.InstallingUserID = installingUserID.String
	}

	return conn, nil
}

// ListWebflowConnections lists all Webflow connections for an organisation
func (db *DB) ListWebflowConnections(ctx context.Context, organisationID string) ([]*WebflowConnection, error) {
	query := `
		SELECT id, organisation_id, webflow_workspace_id, authed_user_id,
		       vault_secret_name, installing_user_id,
		       created_at, updated_at
		FROM webflow_connections
		WHERE organisation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.client.QueryContext(ctx, query, organisationID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to list webflow connections")
		return nil, fmt.Errorf("failed to list webflow connections: %w", err)
	}
	defer rows.Close()

	var connections []*WebflowConnection
	for rows.Next() {
		conn := &WebflowConnection{}
		var installingUserID, vaultSecretName, webflowWorkspaceID, authedUserID sql.NullString

		err := rows.Scan(
			&conn.ID, &conn.OrganisationID, &webflowWorkspaceID, &authedUserID,
			&vaultSecretName, &installingUserID,
			&conn.CreatedAt, &conn.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan webflow connection row")
			return nil, fmt.Errorf("failed to scan webflow connection: %w", err)
		}

		if webflowWorkspaceID.Valid {
			conn.WebflowWorkspaceID = webflowWorkspaceID.String
		}
		if authedUserID.Valid {
			conn.AuthedUserID = authedUserID.String
		}
		if vaultSecretName.Valid {
			conn.VaultSecretName = vaultSecretName.String
		}
		if installingUserID.Valid {
			conn.InstallingUserID = installingUserID.String
		}

		connections = append(connections, conn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating webflow connections: %w", err)
	}

	return connections, nil
}

// DeleteWebflowConnection deletes a Webflow connection
func (db *DB) DeleteWebflowConnection(ctx context.Context, connectionID, organisationID string) error {
	query := `
		DELETE FROM webflow_connections
		WHERE id = $1 AND organisation_id = $2
	`

	result, err := db.client.ExecContext(ctx, query, connectionID, organisationID)
	if err != nil {
		log.Error().Err(err).Str("connection_id", connectionID).Msg("Failed to delete webflow connection")
		return fmt.Errorf("failed to delete webflow connection: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrWebflowConnectionNotFound
	}

	return nil
}
