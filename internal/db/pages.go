package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/lib/pq"
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
	uniqueSet := make(map[string]struct{}, len(batch))
	for _, path := range batch {
		if _, ok := seen[path]; ok {
			continue
		}
		if _, ok := uniqueSet[path]; ok {
			continue
		}
		uniqueSet[path] = struct{}{}
		unique = append(unique, path)
	}

	if len(unique) == 0 {
		return nil
	}

	upsertBatchQuery := `
		WITH batch(path) AS (
			SELECT UNNEST($2::text[])
		)
		INSERT INTO pages (domain_id, path)
		SELECT $1, path
		FROM batch
		ON CONFLICT (domain_id, path)
		DO UPDATE SET path = EXCLUDED.path
		RETURNING path, id
	`

	return q.Execute(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, upsertBatchQuery, domainID, pq.Array(unique))
		if err != nil {
			return fmt.Errorf("failed to upsert page batch: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var path string
			var pageID int
			if err := rows.Scan(&path, &pageID); err != nil {
				return fmt.Errorf("failed to scan upserted page batch row: %w", err)
			}
			seen[path] = pageID
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed during page batch upsert iteration: %w", err)
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
