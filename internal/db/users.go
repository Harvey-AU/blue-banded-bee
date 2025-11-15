package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// User represents a user in the system
type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	FullName       *string   `json:"full_name,omitempty"`
	OrganisationID *string   `json:"organisation_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Organisation represents an organisation in the system
type Organisation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetUser retrieves a user by ID
func (db *DB) GetUser(userID string) (*User, error) {
	user := &User{}

	query := `
		SELECT id, email, full_name, organisation_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	err := db.client.QueryRow(query, userID).Scan(
		&user.ID, &user.Email, &user.FullName, &user.OrganisationID,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByWebhookToken retrieves a user by their webhook token
func (db *DB) GetUserByWebhookToken(webhookToken string) (*User, error) {
	user := &User{}

	query := `
		SELECT id, email, full_name, organisation_id, created_at, updated_at
		FROM users
		WHERE webhook_token = $1
	`

	err := db.client.QueryRow(query, webhookToken).Scan(
		&user.ID, &user.Email, &user.FullName, &user.OrganisationID,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with webhook token")
		}
		return nil, fmt.Errorf("failed to get user by webhook token: %w", err)
	}

	return user, nil
}

// GetOrCreateUser retrieves a user by ID, creating them if they don't exist
// This is used for auto-creating users from valid JWT tokens
func (db *DB) GetOrCreateUser(userID, email string, fullName *string) (*User, error) {
	// First try to get the existing user
	user, err := db.GetUser(userID)
	if err == nil {
		// User exists, return them
		return user, nil
	}

	// User doesn't exist, auto-create them with a default organisation
	log.Info().
		Str("user_id", userID).
		Msg("Auto-creating user from JWT token")

	// Determine organisation name based on email domain
	orgName := deriveOrganisationName(email, fullName)

	// Create the user
	newUser, _, err := db.CreateUser(userID, email, fullName, orgName)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-create user: %w", err)
	}

	return newUser, nil
}

// deriveOrganisationName extracts an organisation name from email and fullName
// Business emails (non-common providers) use domain as org name
// Personal emails (gmail, outlook, etc.) use fullName or "Personal Organisation"
func deriveOrganisationName(email string, fullName *string) string {
	// Common personal email providers
	personalProviders := []string{
		"gmail.com", "googlemail.com",
		"outlook.com", "hotmail.com", "live.com",
		"yahoo.com", "ymail.com",
		"icloud.com", "me.com", "mac.com",
		"protonmail.com", "proton.me",
		"aol.com",
		"zoho.com",
		"fastmail.com",
	}

	// Extract domain from email
	atIndex := strings.LastIndex(email, "@")
	if atIndex == -1 {
		// Invalid email format, fall back to personal
		if fullName != nil && *fullName != "" {
			return *fullName
		}
		return "Personal Organisation"
	}

	emailPrefix := email[:atIndex]
	domain := strings.ToLower(email[atIndex+1:])

	// Check for empty domain (e.g., "user@")
	if domain == "" {
		if fullName != nil && *fullName != "" {
			return *fullName
		}
		// Use email prefix + " Organisation"
		return titleCaseEmailPrefix(emailPrefix) + " Organisation"
	}

	// Check if it's a personal email provider
	for _, provider := range personalProviders {
		if domain == provider {
			// Personal email - use fullName or email prefix
			if fullName != nil && *fullName != "" {
				return *fullName
			}
			return titleCaseEmailPrefix(emailPrefix) + " Organisation"
		}
	}

	// Business email - derive organisation name from domain
	// Remove common TLDs and convert to title case
	orgName := domain

	// Remove TLDs (.com, .co.uk, .com.au, etc.)
	suffixes := []string{".com.au", ".co.uk", ".co.nz", ".com", ".co", ".net", ".org", ".io", ".ai", ".dev"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(orgName, suffix) {
			orgName = strings.TrimSuffix(orgName, suffix)
			break
		}
	}

	// Convert to title case (teamharvey -> Team Harvey)
	// Simple approach: capitalize first letter
	if len(orgName) > 0 {
		orgName = strings.ToUpper(orgName[:1]) + orgName[1:]
	}

	return orgName
}

// isBusinessEmail checks if an email is from a business domain (not a personal email provider)
func isBusinessEmail(email string) bool {
	personalProviders := []string{
		"gmail.com", "googlemail.com",
		"outlook.com", "hotmail.com", "live.com",
		"yahoo.com", "ymail.com",
		"icloud.com", "me.com", "mac.com",
		"protonmail.com", "proton.me",
		"aol.com",
		"zoho.com",
		"fastmail.com",
	}

	atIndex := strings.LastIndex(email, "@")
	if atIndex == -1 {
		return false
	}

	domain := strings.ToLower(email[atIndex+1:])

	for _, provider := range personalProviders {
		if domain == provider {
			return false
		}
	}

	return true
}

// titleCaseEmailPrefix converts email prefix to title case
// Examples: "simon.smallchua" -> "Simon.Smallchua", "user" -> "User"
func titleCaseEmailPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}

	// Split on common separators (., -, _)
	parts := strings.FieldsFunc(prefix, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})

	// Capitalize first letter of each part
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}

	// Rejoin with the original separator (use . for simplicity)
	return strings.Join(parts, ".")
}

// GetOrganisationByName retrieves an organisation by name (case-insensitive)
func (db *DB) GetOrganisationByName(name string) (*Organisation, error) {
	org := &Organisation{}

	query := `
		SELECT id, name, created_at, updated_at
		FROM organisations
		WHERE LOWER(name) = LOWER($1)
		LIMIT 1
	`

	err := db.client.QueryRow(query, name).Scan(
		&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organisation not found")
		}
		return nil, fmt.Errorf("failed to get organisation by name: %w", err)
	}

	return org, nil
}

// CreateOrganisation creates a new organisation
func (db *DB) CreateOrganisation(name string) (*Organisation, error) {
	org := &Organisation{
		ID:   uuid.New().String(),
		Name: name,
	}

	query := `
		INSERT INTO organisations (id, name, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := db.client.QueryRow(query, org.ID, org.Name).Scan(
		&org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create organisation: %w", err)
	}

	log.Info().
		Str("organisation_id", org.ID).
		Str("name", org.Name).
		Msg("Created new organisation")

	return org, nil
}

// GetOrganisation retrieves an organisation by ID
func (db *DB) GetOrganisation(organisationID string) (*Organisation, error) {
	org := &Organisation{}

	query := `
		SELECT id, name, created_at, updated_at
		FROM organisations
		WHERE id = $1
	`

	err := db.client.QueryRow(query, organisationID).Scan(
		&org.ID, &org.Name, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organisation not found")
		}
		return nil, fmt.Errorf("failed to get organisation: %w", err)
	}

	return org, nil
}

func (db *DB) GetOrganisationMembers(organisationID string) ([]*User, error) {
	query := `
		SELECT id, email, full_name, organisation_id, created_at, updated_at
		FROM users
		WHERE organisation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := db.client.Query(query, organisationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organisation members: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.FullName, &user.OrganisationID,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user rows: %w", err)
	}

	return users, nil
}

// If user already exists, returns the existing user and their organisation
func (db *DB) CreateUser(userID, email string, fullName *string, orgName string) (*User, *Organisation, error) {
	// First check if user already exists
	existingUser, err := db.GetUser(userID)
	if err == nil {
		// User exists, get their organisation
		if existingUser.OrganisationID != nil {
			org, err := db.GetOrganisation(*existingUser.OrganisationID)
			if err != nil {
				log.Warn().Err(err).Str("organisation_id", *existingUser.OrganisationID).Msg("Failed to get existing user's organisation")
				// Return user without organisation rather than failing
				return existingUser, nil, nil
			}
			log.Info().
				Str("user_id", userID).
				Msg("User already exists, returning existing user and organisation")
			return existingUser, org, nil
		}
		// User exists but has no organisation - this shouldn't happen but handle gracefully
		log.Info().
			Str("user_id", userID).
			Msg("User already exists but has no organisation")
		return existingUser, nil, nil
	}

	// User doesn't exist, create new user and organisation
	// Start a transaction for automatic operation
	tx, err := db.client.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback is safe to call even after commit
	}()

	// For business emails, check if organisation already exists
	var org *Organisation
	if isBusinessEmail(email) {
		existingOrg, err := db.GetOrganisationByName(orgName)
		if err == nil {
			// Organisation exists, use it
			org = existingOrg
			log.Info().
				Str("user_id", userID).
				Str("organisation_id", org.ID).
				Str("organisation_name", org.Name).
				Msg("Joining existing organisation (business email)")
		}
	}

	// If no existing organisation found (or personal email), create new one
	if org == nil {
		org = &Organisation{
			ID:   uuid.New().String(),
			Name: orgName,
		}

		orgQuery := `
			INSERT INTO organisations (id, name, created_at, updated_at)
			VALUES ($1, $2, NOW(), NOW())
			RETURNING created_at, updated_at
		`

		err = tx.QueryRow(orgQuery, org.ID, org.Name).Scan(
			&org.CreatedAt, &org.UpdatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create organisation: %w", err)
		}
	}

	// Create user with organisation reference
	user := &User{
		ID:             userID,
		Email:          email,
		FullName:       fullName,
		OrganisationID: &org.ID,
	}

	userQuery := `
		INSERT INTO users (id, email, full_name, organisation_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err = tx.QueryRow(userQuery, user.ID, user.Email, user.FullName, user.OrganisationID).Scan(
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("user_id", user.ID).
		Str("organisation_id", org.ID).
		Str("organisation_name", org.Name).
		Msg("Created new user with organisation")

	return user, org, nil
}
