package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Harvey-AU/blue-banded-bee/internal/util"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
)

// QueueProvider defines an interface for accessing database operations
type QueueProvider interface {
	Execute(ctx context.Context, fn func(*sql.Tx) error) error
}

// CreatePageRecords creates page records for a list of URLs and returns their IDs and paths
// This function extracts paths from full URLs and creates database records
func CreatePageRecords(ctx context.Context, dbQueue QueueProvider, domainID int, urls []string) ([]int, []string, error) {
	span := sentry.StartSpan(ctx, "db.create_page_records")
	defer span.Finish()

	span.SetTag("domain_id", fmt.Sprintf("%d", domainID))
	span.SetTag("url_count", fmt.Sprintf("%d", len(urls)))

	if len(urls) == 0 {
		return []int{}, []string{}, nil
	}

	pageIDs := make([]int, 0, len(urls))
	paths := make([]string, 0, len(urls))

	// Get domain name from the database
	var domainName string
	err := dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT name FROM domains WHERE id = $1
		`, domainID).Scan(&domainName)
	})
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, nil, fmt.Errorf("failed to get domain name: %w", err)
	}

	// Extract paths from URLs
	for _, url := range urls {
		// Parse URL to extract the path
		log.Debug().Str("original_url", url).Msg("Processing URL")

		// Use centralized URL utility to extract path
		path := util.ExtractPathFromURL(url)

		// Add paths to our result array
		paths = append(paths, path)
		log.Debug().Str("extracted_path", path).Msg("Extracted path from URL")
	}

	// Insert pages into database in a transaction
	err = dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// Prepare statement for bulk insertion
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare page insert statement: %w", err)
		}
		defer stmt.Close()

		// Insert each page and collect the IDs
		for _, path := range paths {
			var pageID int
			err := stmt.QueryRowContext(ctx, domainID, path).Scan(&pageID)
			if err != nil {
				return fmt.Errorf("failed to insert page record: %w", err)
			}
			pageIDs = append(pageIDs, pageID)
		}

		return nil
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, nil, fmt.Errorf("failed to create page records: %w", err)
	}

	log.Debug().
		Int("domain_id", domainID).
		Int("page_count", len(pageIDs)).
		Msg("Created page records")

	return pageIDs, paths, nil
}