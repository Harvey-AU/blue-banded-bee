package api

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// loggerWithRequest returns a logger enriched with request context so that all
// API logs include correlation identifiers without repeating boilerplate.
func loggerWithRequest(r *http.Request) zerolog.Logger {
	if r == nil {
		return log.With().Logger()
	}

	builder := log.With().
		Str("request_id", GetRequestID(r)).
		Str("method", r.Method).
		Str("path", r.URL.Path)

	return builder.Logger()
}
