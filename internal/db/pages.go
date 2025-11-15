package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

const maxPageRecordBatchSize = 250

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
	if len(urls) == 0 {
		return nil, nil, nil
	}

	var pageIDs []int
	var paths []string
	seen := make(map[string]int, len(urls))
	batched := make([]string, 0, maxPageRecordBatchSize)

	flushBatch := func() error {
		if len(batched) == 0 {
			return nil
		}

		if err := ensurePageBatch(ctx, q, domainID, batched, seen); err != nil {
			return err
		}

		for _, path := range batched {
			id, ok := seen[path]
			if !ok {
				continue
			}
			pageIDs = append(pageIDs, id)
			paths = append(paths, path)
		}

		batched = batched[:0]
		return nil
	}

	for _, u := range urls {
		path, err := normaliseURLPath(u, domain)
		if err != nil {
			log.Warn().Err(err).Str("url", u).Msg("Skipping invalid URL")
			continue
		}

		if id, ok := seen[path]; ok {
			pageIDs = append(pageIDs, id)
			paths = append(paths, path)
			continue
		}

		batched = append(batched, path)
		if len(batched) >= maxPageRecordBatchSize {
			if err := flushBatch(); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := flushBatch(); err != nil {
		return nil, nil, err
	}

	return pageIDs, paths, nil
}

func ensurePageBatch(ctx context.Context, q TransactionExecutor, domainID int, batch []string, seen map[string]int) error {
	unique := make([]string, 0, len(batch))
	for _, path := range batch {
		if _, ok := seen[path]; ok {
			continue
		}
		unique = append(unique, path)
	}

	if len(unique) == 0 {
		return nil
	}

	insertQuery := `
		INSERT INTO pages (domain_id, path)
		VALUES ($1, $2)
		ON CONFLICT (domain_id, path)
		DO UPDATE SET path = EXCLUDED.path
		RETURNING id
	`

	return q.Execute(ctx, func(tx *sql.Tx) error {
		for _, path := range unique {
			var pageID int
			if err := tx.QueryRowContext(ctx, insertQuery, domainID, path).Scan(&pageID); err != nil {
				return fmt.Errorf("failed to upsert page record: %w", err)
			}

			seen[path] = pageID
		}
		return nil
	})
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
