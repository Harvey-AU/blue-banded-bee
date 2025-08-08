package db

import (
	"fmt"
	"strings"
)

// AugmentDSNWithTimeout adds statement_timeout to a DSN if not already present
// Supports both URL format (postgresql://...) and key=value format
func AugmentDSNWithTimeout(dsn string, timeoutMs int) string {
	if dsn == "" || strings.Contains(dsn, "statement_timeout") {
		return dsn
	}

	if timeoutMs <= 0 {
		timeoutMs = 60000 // Default 60 seconds
	}
	timeoutStr := fmt.Sprintf("%d", timeoutMs)

	// URL format
	if strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "postgres://") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		return dsn + separator + "statement_timeout=" + timeoutStr
	}

	// Key=value format
	separator := " "
	return dsn + separator + "statement_timeout=" + timeoutStr
}