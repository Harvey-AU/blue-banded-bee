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

// ErrGoogleAccountNotFound is returned when a Google Analytics account is not found
var ErrGoogleAccountNotFound = errors.New("google analytics account not found")

// GoogleAnalyticsAccount represents an organisation's linked Google Analytics account
// Accounts are synced from Google API and stored in DB for persistent display.
type GoogleAnalyticsAccount struct {
	ID                string // UUID
	OrganisationID    string // Organisation this account belongs to
	GoogleAccountID   string // GA account ID (e.g., "accounts/123456")
	GoogleAccountName string // Display name of the account
	GoogleUserID      string // Google user ID who authorised
	GoogleEmail       string // Google email for display
	VaultSecretName   string // Token stored in Vault
	InstallingUserID  string // Our user who installed
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

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
	if conn.DomainIDs == nil {
		conn.DomainIDs = pq.Int64Array{}
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
		if conn.DomainIDs == nil {
			conn.DomainIDs = pq.Int64Array{}
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
		ORDER BY last_synced_at DESC NULLS LAST, updated_at DESC, created_at DESC, id ASC
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
	if conn.DomainIDs == nil {
		conn.DomainIDs = pq.Int64Array{}
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
		ORDER BY last_synced_at DESC NULLS LAST, updated_at DESC, created_at DESC, id ASC
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
	if conn.DomainIDs == nil {
		conn.DomainIDs = pq.Int64Array{}
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

// UpdateConnectionDomains updates the domain_ids for an existing connection
func (db *DB) UpdateConnectionDomains(ctx context.Context, connectionID string, domainIDs []int) error {
	// Convert []int to pq.Int64Array
	var domainIDsArray pq.Int64Array
	if domainIDs != nil {
		domainIDsArray = make(pq.Int64Array, len(domainIDs))
		for i, id := range domainIDs {
			domainIDsArray[i] = int64(id)
		}
	}

	query := `
		UPDATE google_analytics_connections
		SET domain_ids = $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	result, err := db.client.ExecContext(ctx, query, pq.Array(domainIDsArray), connectionID)
	if err != nil {
		return fmt.Errorf("failed to update connection domains: %w", err)
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

// MarkConnectionInactive sets a connection status to inactive with a reason logged
func (db *DB) MarkConnectionInactive(ctx context.Context, connectionID, reason string) error {
	query := `
		UPDATE google_analytics_connections
		SET status = 'inactive', inactive_reason = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := db.client.ExecContext(ctx, query, connectionID, reason)
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
		Str("next_action", "reauthorise_google_connection").
		Msg("Marked Google Analytics connection as inactive")

	return nil
}

// ============================================================================
// Google Analytics Accounts (for persistent account storage)
// ============================================================================

// UpsertGA4Account creates or updates a Google Analytics account record
func (db *DB) UpsertGA4Account(ctx context.Context, account *GoogleAnalyticsAccount) error {
	query := `
		INSERT INTO google_analytics_accounts (
			id, organisation_id, google_account_id, google_account_name,
			google_user_id, google_email, installing_user_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organisation_id, google_account_id)
		DO UPDATE SET
			google_account_name = EXCLUDED.google_account_name,
			google_user_id = EXCLUDED.google_user_id,
			google_email = EXCLUDED.google_email,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	err := db.client.QueryRowContext(ctx, query,
		account.ID, account.OrganisationID, account.GoogleAccountID, account.GoogleAccountName,
		account.GoogleUserID, account.GoogleEmail, account.InstallingUserID,
		account.CreatedAt, account.UpdatedAt,
	).Scan(&account.ID)
	if err != nil {
		log.Error().Err(err).
			Str("organisation_id", account.OrganisationID).
			Str("google_account_id", account.GoogleAccountID).
			Msg("Failed to upsert Google Analytics account")
		return fmt.Errorf("failed to upsert Google Analytics account: %w", err)
	}

	return nil
}

// ListGA4Accounts lists all Google Analytics accounts for an organisation
func (db *DB) ListGA4Accounts(ctx context.Context, organisationID string) ([]*GoogleAnalyticsAccount, error) {
	query := `
		SELECT id, organisation_id, google_account_id, google_account_name,
		       google_user_id, google_email, vault_secret_name, installing_user_id,
		       created_at, updated_at
		FROM google_analytics_accounts
		WHERE organisation_id = $1
		ORDER BY google_account_name ASC
	`

	rows, err := db.client.QueryContext(ctx, query, organisationID)
	if err != nil {
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to list Google Analytics accounts")
		return nil, fmt.Errorf("failed to list Google Analytics accounts: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Error().Err(closeErr).Str("organisation_id", organisationID).Msg("Failed to close rows")
		}
	}()

	var accounts []*GoogleAnalyticsAccount
	for rows.Next() {
		acc := &GoogleAnalyticsAccount{}
		var googleAccountName, googleUserID, googleEmail, vaultSecretName, installingUserID sql.NullString

		err := rows.Scan(
			&acc.ID, &acc.OrganisationID, &acc.GoogleAccountID, &googleAccountName,
			&googleUserID, &googleEmail, &vaultSecretName, &installingUserID,
			&acc.CreatedAt, &acc.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to scan Google Analytics account row")
			return nil, fmt.Errorf("failed to scan Google Analytics account: %w", err)
		}

		if googleAccountName.Valid {
			acc.GoogleAccountName = googleAccountName.String
		}
		if googleUserID.Valid {
			acc.GoogleUserID = googleUserID.String
		}
		if googleEmail.Valid {
			acc.GoogleEmail = googleEmail.String
		}
		if vaultSecretName.Valid {
			acc.VaultSecretName = vaultSecretName.String
		}
		if installingUserID.Valid {
			acc.InstallingUserID = installingUserID.String
		}

		accounts = append(accounts, acc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating Google Analytics accounts: %w", err)
	}

	return accounts, nil
}

// GetGA4Account retrieves a Google Analytics account by ID
func (db *DB) GetGA4Account(ctx context.Context, accountID string) (*GoogleAnalyticsAccount, error) {
	acc := &GoogleAnalyticsAccount{}
	var googleAccountName, googleUserID, googleEmail, vaultSecretName, installingUserID sql.NullString

	query := `
		SELECT id, organisation_id, google_account_id, google_account_name,
		       google_user_id, google_email, vault_secret_name, installing_user_id,
		       created_at, updated_at
		FROM google_analytics_accounts
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, accountID).Scan(
		&acc.ID, &acc.OrganisationID, &acc.GoogleAccountID, &googleAccountName,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoogleAccountNotFound
		}
		log.Error().Err(err).Str("account_id", accountID).Msg("Failed to get Google Analytics account")
		return nil, fmt.Errorf("failed to get Google Analytics account: %w", err)
	}

	if googleAccountName.Valid {
		acc.GoogleAccountName = googleAccountName.String
	}
	if googleUserID.Valid {
		acc.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		acc.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		acc.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		acc.InstallingUserID = installingUserID.String
	}

	return acc, nil
}

// GetGA4AccountByGoogleID retrieves a Google Analytics account by its Google account ID
func (db *DB) GetGA4AccountByGoogleID(ctx context.Context, organisationID, googleAccountID string) (*GoogleAnalyticsAccount, error) {
	acc := &GoogleAnalyticsAccount{}
	var googleAccountName, googleUserID, googleEmail, vaultSecretName, installingUserID sql.NullString

	query := `
		SELECT id, organisation_id, google_account_id, google_account_name,
		       google_user_id, google_email, vault_secret_name, installing_user_id,
		       created_at, updated_at
		FROM google_analytics_accounts
		WHERE organisation_id = $1 AND google_account_id = $2
	`

	err := db.client.QueryRowContext(ctx, query, organisationID, googleAccountID).Scan(
		&acc.ID, &acc.OrganisationID, &acc.GoogleAccountID, &googleAccountName,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoogleAccountNotFound
		}
		log.Error().Err(err).
			Str("organisation_id", organisationID).
			Str("google_account_id", googleAccountID).
			Msg("Failed to get Google Analytics account by Google ID")
		return nil, fmt.Errorf("failed to get Google Analytics account: %w", err)
	}

	if googleAccountName.Valid {
		acc.GoogleAccountName = googleAccountName.String
	}
	if googleUserID.Valid {
		acc.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		acc.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		acc.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		acc.InstallingUserID = installingUserID.String
	}

	return acc, nil
}

// StoreGA4AccountToken stores a Google Analytics refresh token for an account in Supabase Vault
func (db *DB) StoreGA4AccountToken(ctx context.Context, accountID, refreshToken string) error {
	// Use a dedicated vault function for accounts (similar to store_ga_token for connections)
	// For now, we store it in the same vault with account-specific prefix
	query := `SELECT store_ga_account_token($1::uuid, $2)`

	if err := db.client.QueryRowContext(ctx, query, accountID, refreshToken).Scan(new(string)); err != nil {
		log.Error().Err(err).Str("account_id", accountID).Msg("Failed to store GA account token in vault")
		return fmt.Errorf("failed to store GA account token: %w", err)
	}

	return nil
}

// GetGA4AccountToken retrieves a Google Analytics refresh token for an account from Supabase Vault
func (db *DB) GetGA4AccountToken(ctx context.Context, accountID string) (string, error) {
	query := `SELECT get_ga_account_token($1::uuid)`

	var token sql.NullString
	err := db.client.QueryRowContext(ctx, query, accountID).Scan(&token)
	if err != nil {
		log.Error().Err(err).Str("account_id", accountID).Msg("Failed to get GA account token from vault")
		return "", fmt.Errorf("failed to get GA account token: %w", err)
	}

	if !token.Valid {
		return "", ErrGoogleTokenNotFound
	}

	return token.String, nil
}

// GetGA4AccountWithToken retrieves a GA4 account that has a valid token stored
// Returns the first account found with a token for the organisation
func (db *DB) GetGA4AccountWithToken(ctx context.Context, organisationID string) (*GoogleAnalyticsAccount, error) {
	query := `
		SELECT a.id, a.organisation_id, a.google_account_id, a.google_account_name,
		       a.google_user_id, a.google_email, a.vault_secret_name, a.installing_user_id,
		       a.created_at, a.updated_at
		FROM google_analytics_accounts a
		WHERE a.organisation_id = $1
		  AND a.vault_secret_name IS NOT NULL
		ORDER BY a.updated_at DESC
		LIMIT 1
	`

	acc := &GoogleAnalyticsAccount{}
	var googleAccountName, googleUserID, googleEmail, vaultSecretName, installingUserID sql.NullString

	err := db.client.QueryRowContext(ctx, query, organisationID).Scan(
		&acc.ID, &acc.OrganisationID, &acc.GoogleAccountID, &googleAccountName,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoogleAccountNotFound
		}
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to get GA4 account with token")
		return nil, fmt.Errorf("failed to get GA4 account with token: %w", err)
	}

	if googleAccountName.Valid {
		acc.GoogleAccountName = googleAccountName.String
	}
	if googleUserID.Valid {
		acc.GoogleUserID = googleUserID.String
	}
	if googleEmail.Valid {
		acc.GoogleEmail = googleEmail.String
	}
	if vaultSecretName.Valid {
		acc.VaultSecretName = vaultSecretName.String
	}
	if installingUserID.Valid {
		acc.InstallingUserID = installingUserID.String
	}

	return acc, nil
}

// GetGAConnectionWithToken retrieves a GA4 connection that has a valid token stored
// Returns the most recently updated connection with a token for the organisation
func (db *DB) GetGAConnectionWithToken(ctx context.Context, organisationID string) (*GoogleAnalyticsConnection, error) {
	query := `
		SELECT id, organisation_id, ga4_property_id, ga4_property_name, google_account_id,
		       google_user_id, google_email, vault_secret_name, installing_user_id, status,
		       domain_ids, last_synced_at, created_at, updated_at
		FROM google_analytics_connections
		WHERE organisation_id = $1
		  AND vault_secret_name IS NOT NULL
		ORDER BY updated_at DESC
		LIMIT 1
	`

	conn := &GoogleAnalyticsConnection{}
	var installingUserID, vaultSecretName, ga4PropertyID, ga4PropertyName, googleAccountID, googleUserID, googleEmail, status sql.NullString
	var lastSyncedAt sql.NullTime

	err := db.client.QueryRowContext(ctx, query, organisationID).Scan(
		&conn.ID, &conn.OrganisationID, &ga4PropertyID, &ga4PropertyName, &googleAccountID,
		&googleUserID, &googleEmail, &vaultSecretName, &installingUserID, &status,
		&conn.DomainIDs, &lastSyncedAt, &conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoogleConnectionNotFound
		}
		log.Error().Err(err).Str("organisation_id", organisationID).Msg("Failed to get GA4 connection with token")
		return nil, fmt.Errorf("failed to get GA4 connection with token: %w", err)
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
	if conn.DomainIDs == nil {
		conn.DomainIDs = pq.Int64Array{}
	}

	return conn, nil
}
