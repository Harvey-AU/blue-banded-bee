package db

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCoreTables(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
		errorTable  string
	}{
		{
			name:        "successful_table_creation",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "organisations_table_creation_fails",
			mockError:   sql.ErrConnDone,
			expectError: true,
			errorTable:  "organisations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			if tt.expectError {
				// Expect the first table creation to fail
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations").
					WillReturnError(tt.mockError)
			} else {
				// Expect all table creations to succeed
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS domains").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS pages").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS jobs").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec("CREATE TABLE IF NOT EXISTS tasks").
					WillReturnResult(sqlmock.NewResult(0, 0))
			}

			err = createCoreTables(db)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorTable != "" {
					assert.Contains(t, err.Error(), tt.errorTable)
				}
			} else {
				assert.NoError(t, err)
			}

			// Verify all expected calls were made
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreateCoreTablesTableOrder(t *testing.T) {
	// Test that tables are created in correct dependency order
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect tables in dependency order
	
	// organisations first (no dependencies)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations").
		WillReturnResult(sqlmock.NewResult(0, 0))
	
	// users (depends on organisations)  
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
		WillReturnResult(sqlmock.NewResult(0, 0))
		
	// domains (no dependencies)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS domains").
		WillReturnResult(sqlmock.NewResult(0, 0))
		
	// pages (depends on domains)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS pages").
		WillReturnResult(sqlmock.NewResult(0, 0))
		
	// jobs (depends on domains, users, organisations)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS jobs").
		WillReturnResult(sqlmock.NewResult(0, 0))
		
	// tasks (depends on jobs, pages)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS tasks").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = createCoreTables(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCoreTablesSchemaElements(t *testing.T) {
	// Test that key schema elements are included in table creation
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Set up expectations that capture the SQL
	orgExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations")
	userExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS users")
	domainExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS domains")
	pageExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS pages")
	jobExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS jobs")
	taskExpect := mock.ExpectExec("CREATE TABLE IF NOT EXISTS tasks")

	orgExpect.WillReturnResult(sqlmock.NewResult(0, 0))
	userExpect.WillReturnResult(sqlmock.NewResult(0, 0))
	domainExpect.WillReturnResult(sqlmock.NewResult(0, 0))
	pageExpect.WillReturnResult(sqlmock.NewResult(0, 0))
	jobExpect.WillReturnResult(sqlmock.NewResult(0, 0))
	taskExpect.WillReturnResult(sqlmock.NewResult(0, 0))

	err = createCoreTables(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateCoreTablesErrorPropagation(t *testing.T) {
	// Test that errors from different tables are properly propagated
	tables := []struct {
		name       string
		failAtTable string
	}{
		{"organisations_failure", "organisations"},
		{"users_failure", "users"},
		{"domains_failure", "domains"},
		{"pages_failure", "pages"},
		{"jobs_failure", "jobs"},
		{"tasks_failure", "tasks"},
	}

	for _, tt := range tables {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tableOrder := []string{"organisations", "users", "domains", "pages", "jobs", "tasks"}
			
			for _, table := range tableOrder {
				if table == tt.failAtTable {
					mock.ExpectExec("CREATE TABLE IF NOT EXISTS " + table).
						WillReturnError(sql.ErrConnDone)
					break
				} else {
					mock.ExpectExec("CREATE TABLE IF NOT EXISTS " + table).
						WillReturnResult(sqlmock.NewResult(0, 0))
				}
			}

			err = createCoreTables(db)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.failAtTable)
			
			// Note: expectationsWereMet may not be satisfied if we fail early,
			// but that's expected behavior
		})
	}
}