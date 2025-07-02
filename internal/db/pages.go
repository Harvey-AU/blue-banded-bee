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

// CreatePageRecords finds existing pages or creates new ones for the given URLs.
// It returns the page IDs and their corresponding paths.
func CreatePageRecords(ctx context.Context, q *DbQueue, domainID int, domain string, urls []string) ([]int, []string, error) {
	var pageIDs []int
	var paths []string

	err := q.Execute(ctx, func(tx *sql.Tx) error {
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

		for _, u := range urls {
			// Normalise URL to get just the path
			path, err := normaliseURLPath(u, domain)
			if err != nil {
				log.Warn().Err(err).Str("url", u).Msg("Skipping invalid URL")
				continue
			}

			var pageID int
			if err := stmt.QueryRowContext(ctx, domainID, path).Scan(&pageID); err != nil {
				return fmt.Errorf("failed to insert/get page record: %w", err)
			}
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
