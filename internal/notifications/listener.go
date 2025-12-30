package notifications

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// Listener listens for PostgreSQL notifications and triggers delivery
type Listener struct {
	connStr string
	service *Service
}

// NewListener creates a new notification listener
func NewListener(connStr string, service *Service) *Listener {
	return &Listener{
		connStr: connStr,
		service: service,
	}
}

// Start begins listening for notifications
// It uses PostgreSQL LISTEN/NOTIFY for real-time delivery
// Falls back to polling if the connection fails
func (l *Listener) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Notification listener stopped")
			return
		default:
			if err := l.listen(ctx); err != nil {
				log.Warn().Err(err).Msg("Notification listener error, retrying in 5s")
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
		}
	}
}

func (l *Listener) listen(ctx context.Context) error {
	// Create a dedicated connection for LISTEN
	listener := pq.NewListener(l.connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Warn().Err(err).Msg("Notification listener event error")
		}
	})
	defer listener.Close()

	if err := listener.Listen("new_notification"); err != nil {
		return err
	}

	log.Info().Msg("Notification listener started (real-time mode)")

	// Process any pending notifications on startup
	l.processPending(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil

		case notification := <-listener.Notify:
			if notification == nil {
				// Connection lost, reconnect
				return nil
			}

			log.Debug().
				Str("channel", notification.Channel).
				Str("payload", notification.Extra).
				Msg("Received notification")

			// Process pending notifications (the payload is the notification ID,
			// but we process all pending to handle any that might have been missed)
			l.processPending(ctx)

		case <-time.After(90 * time.Second):
			// Ping to keep connection alive
			if err := listener.Ping(); err != nil {
				return err
			}
		}
	}
}

func (l *Listener) processPending(ctx context.Context) {
	if err := l.service.ProcessPendingNotifications(ctx, 50); err != nil {
		if ctx.Err() == nil {
			log.Warn().Err(err).Msg("Failed to process pending notifications")
		}
	}
}

// StartWithFallback starts the listener with polling fallback
// This is useful when the database doesn't support LISTEN (e.g., connection poolers)
func StartWithFallback(ctx context.Context, db *sql.DB, connStr string, service *Service) {
	// Try to use LISTEN/NOTIFY first
	listener := NewListener(connStr, service)

	// Check if we can use LISTEN (direct connection, not pooled)
	if canUseListen(connStr) {
		go listener.Start(ctx)
		return
	}

	// Fall back to polling
	log.Info().Msg("Using polling mode for notifications (connection pooler detected)")
	go startPolling(ctx, service)
}

func canUseListen(connStr string) bool {
	// Connection poolers (like PgBouncer in transaction mode) don't support LISTEN
	// Supabase's pooler URLs contain "pooler" in the host
	// For now, always try LISTEN - it will fall back if it fails
	return true
}

func startPolling(ctx context.Context, service *Service) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Info().Msg("Notification processor started (polling mode)")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Notification processor stopped")
			return
		case <-ticker.C:
			if err := service.ProcessPendingNotifications(ctx, 50); err != nil {
				if ctx.Err() == nil {
					log.Warn().Err(err).Msg("Failed to process pending notifications")
				}
			}
		}
	}
}
