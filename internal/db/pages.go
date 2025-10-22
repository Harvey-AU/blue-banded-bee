package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// Page represents a page to be enqueued with its priority
type Page struct {
	ID       int
	Path     string
	Priority float64
}

// TransactionExecutor interface for types that can execute transactions
type TransactionExecutor interface {
	Execute(ctx context.Context, fn func(*sql.Tx) error) error
}

// CreatePageRecords finds existing pages or creates new ones for the given URLs.
// It returns the page IDs and their corresponding paths.
func CreatePageRecords(ctx context.Context, q TransactionExecutor, domainID int, domain string, urls []string) ([]int, []string, error) {
	var pageIDs []int
	var paths []string

	err := q.Execute(ctx, func(tx *sql.Tx) error {
		// Use direct queries instead of prepared statements for Supabase pooler compatibility
		insertQuery := `
			INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO NOTHING
			RETURNING id
		`
		selectQuery := `
			SELECT id FROM pages WHERE domain_id = $1 AND path = $2
		`

		seen := make(map[string]int, len(urls))

		for _, u := range urls {
			// Normalise URL to get just the path
			path, err := normaliseURLPath(u, domain)
			if err != nil {
				log.Warn().Err(err).Str("url", u).Msg("Skipping invalid URL")
				continue
			}

			// Skip duplicates within this batch and reuse cached IDs
			if id, ok := seen[path]; ok {
				pageIDs = append(pageIDs, id)
				paths = append(paths, path)
				continue
			}

			// Get or create the page record without touching existing rows unnecessarily
			var pageID int
			err = tx.QueryRowContext(ctx, insertQuery, domainID, path).Scan(&pageID)
			if err == sql.ErrNoRows {
				if err := tx.QueryRowContext(ctx, selectQuery, domainID, path).Scan(&pageID); err != nil {
					return fmt.Errorf("failed to lookup existing page record: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("failed to insert page record: %w", err)
			}

			seen[path] = pageID
			pageIDs = append(pageIDs, pageID)
			paths = append(paths, path)
		}
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return pageIDs, paths, nil
}

func normaliseURLPath(u string, domain string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	if !parsedURL.IsAbs() {
		base, _ := url.Parse("https://" + domain)
		parsedURL = base.ResolveReference(parsedURL)
	}
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path, nil
}
