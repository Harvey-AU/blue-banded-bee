package db

import (
	"database/sql"
	"fmt"
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

// GetOrganisationMembers retrieves all members of an organisation
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

// CreateUserWithOrganisation creates a new user and organisation atomically
// If user already exists, returns the existing user and their organisation
func (db *DB) CreateUserWithOrganisation(userID, email string, fullName *string, orgName string) (*User, *Organisation, error) {
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
				Str("email", email).
				Msg("User already exists, returning existing user and organisation")
			return existingUser, org, nil
		}
		// User exists but has no organisation - this shouldn't happen but handle gracefully
		log.Info().
			Str("user_id", userID).
			Str("email", email).
			Msg("User already exists but has no organisation")
		return existingUser, nil, nil
	}

	// User doesn't exist, create new user and organisation
	// Start a transaction for atomic operation
	tx, err := db.client.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Create organisation first
	org := &Organisation{
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
		Str("email", user.Email).
		Str("organisation_id", org.ID).
		Str("organisation_name", org.Name).
		Msg("Created new user with organisation")

	return user, org, nil
}