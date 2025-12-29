package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// NotificationType defines the types of notifications
type NotificationType string

const (
	NotificationJobComplete    NotificationType = "job_complete"
	NotificationJobFailed      NotificationType = "job_failed"
	NotificationSchedulerRun   NotificationType = "scheduler_run"
	NotificationSchedulerError NotificationType = "scheduler_error"
)

// Notification represents a notification record
type Notification struct {
	ID             string
	OrganisationID string
	UserID         *string // nil for org-wide notifications
	Type           NotificationType
	Title          string
	Message        string
	Data           map[string]interface{}
	IsRead         bool
	DeliveredSlack bool
	DeliveredEmail bool
	CreatedAt      time.Time
}

// NotificationData is the structured payload for different notification types
type NotificationData struct {
	JobID          string `json:"job_id,omitempty"`
	Domain         string `json:"domain,omitempty"`
	CompletedTasks int    `json:"completed_tasks,omitempty"`
	FailedTasks    int    `json:"failed_tasks,omitempty"`
	Duration       string `json:"duration,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
	SchedulerID    string `json:"scheduler_id,omitempty"`
}

// CreateNotification inserts a new notification
func (db *DB) CreateNotification(ctx context.Context, n *Notification) error {
	dataJSON, err := json.Marshal(n.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal notification data: %w", err)
	}

	query := `
		INSERT INTO notifications (
			id, organisation_id, user_id, type, title, message, data, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = db.client.ExecContext(ctx, query,
		n.ID, n.OrganisationID, n.UserID, n.Type, n.Title, n.Message, dataJSON, n.CreatedAt,
	)
	if err != nil {
		log.Error().Err(err).Str("notification_id", n.ID).Msg("Failed to create notification")
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// GetNotification retrieves a notification by ID
func (db *DB) GetNotification(ctx context.Context, notificationID string) (*Notification, error) {
	n := &Notification{}
	var userID sql.NullString
	var message sql.NullString
	var dataJSON []byte

	query := `
		SELECT id, organisation_id, user_id, type, title, message, data,
		       is_read, delivered_slack, delivered_email, created_at
		FROM notifications
		WHERE id = $1
	`

	err := db.client.QueryRowContext(ctx, query, notificationID).Scan(
		&n.ID, &n.OrganisationID, &userID, &n.Type, &n.Title, &message, &dataJSON,
		&n.IsRead, &n.DeliveredSlack, &n.DeliveredEmail, &n.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	if userID.Valid {
		n.UserID = &userID.String
	}
	if message.Valid {
		n.Message = message.String
	}
	if dataJSON != nil {
		if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
			log.Warn().Err(err).Str("notification_id", notificationID).Msg("Failed to unmarshal notification data")
		}
	}

	return n, nil
}

// ListNotifications retrieves notifications for an organisation
func (db *DB) ListNotifications(ctx context.Context, organisationID string, limit, offset int, unreadOnly bool) ([]*Notification, int, error) {
	var whereClause string
	args := []interface{}{organisationID}
	argIndex := 2

	if unreadOnly {
		whereClause = "WHERE organisation_id = $1 AND is_read = false"
	} else {
		whereClause = "WHERE organisation_id = $1"
	}

	// Count total
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM notifications %s`, whereClause)
	var total int
	if err := db.client.QueryRowContext(ctx, countQuery, organisationID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Fetch notifications
	query := fmt.Sprintf(`
		SELECT id, organisation_id, user_id, type, title, message, data,
		       is_read, delivered_slack, delivered_email, created_at
		FROM notifications
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args = append(args, limit, offset)

	rows, err := db.client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		var userID sql.NullString
		var message sql.NullString
		var dataJSON []byte

		err := rows.Scan(
			&n.ID, &n.OrganisationID, &userID, &n.Type, &n.Title, &message, &dataJSON,
			&n.IsRead, &n.DeliveredSlack, &n.DeliveredEmail, &n.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}

		if userID.Valid {
			n.UserID = &userID.String
		}
		if message.Valid {
			n.Message = message.String
		}
		if dataJSON != nil {
			n.Data = make(map[string]interface{})
			_ = json.Unmarshal(dataJSON, &n.Data) // Error ignored: best-effort JSON parsing
		}

		notifications = append(notifications, n)
	}

	return notifications, total, nil
}

// MarkNotificationRead marks a notification as read
func (db *DB) MarkNotificationRead(ctx context.Context, notificationID, organisationID string) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND organisation_id = $2
	`

	result, err := db.client.ExecContext(ctx, query, notificationID, organisationID)
	if err != nil {
		return fmt.Errorf("failed to mark notification read: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}

	return nil
}

// MarkAllNotificationsRead marks all notifications as read for an organisation
func (db *DB) MarkAllNotificationsRead(ctx context.Context, organisationID string) error {
	query := `
		UPDATE notifications
		SET is_read = true
		WHERE organisation_id = $1 AND is_read = false
	`

	_, err := db.client.ExecContext(ctx, query, organisationID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications read: %w", err)
	}

	return nil
}

// GetPendingSlackNotifications retrieves notifications not yet delivered to Slack
func (db *DB) GetPendingSlackNotifications(ctx context.Context, limit int) ([]*Notification, error) {
	query := `
		SELECT n.id, n.organisation_id, n.user_id, n.type, n.title, n.message, n.data,
		       n.is_read, n.delivered_slack, n.delivered_email, n.created_at
		FROM notifications n
		WHERE n.delivered_slack = false
		  AND EXISTS (SELECT 1 FROM slack_connections sc WHERE sc.organisation_id = n.organisation_id)
		ORDER BY n.created_at ASC
		LIMIT $1
	`

	rows, err := db.client.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending Slack notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		var userID sql.NullString
		var message sql.NullString
		var dataJSON []byte

		err := rows.Scan(
			&n.ID, &n.OrganisationID, &userID, &n.Type, &n.Title, &message, &dataJSON,
			&n.IsRead, &n.DeliveredSlack, &n.DeliveredEmail, &n.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		if userID.Valid {
			n.UserID = &userID.String
		}
		if message.Valid {
			n.Message = message.String
		}
		if dataJSON != nil {
			n.Data = make(map[string]interface{})
			_ = json.Unmarshal(dataJSON, &n.Data) // Error ignored: best-effort JSON parsing
		}

		notifications = append(notifications, n)
	}

	return notifications, nil
}

// MarkNotificationDelivered marks a notification as delivered to a channel
func (db *DB) MarkNotificationDelivered(ctx context.Context, notificationID, channel string) error {
	var column string
	switch channel {
	case "slack":
		column = "delivered_slack"
	case "email":
		column = "delivered_email"
	default:
		return fmt.Errorf("unknown channel: %s", channel)
	}

	query := fmt.Sprintf(`
		UPDATE notifications
		SET %s = true
		WHERE id = $1
	`, column)

	_, err := db.client.ExecContext(ctx, query, notificationID)
	if err != nil {
		return fmt.Errorf("failed to mark notification delivered: %w", err)
	}

	return nil
}

// GetUnreadCount returns the count of unread notifications for an organisation
func (db *DB) GetUnreadNotificationCount(ctx context.Context, organisationID string) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE organisation_id = $1 AND is_read = false`

	var count int
	err := db.client.QueryRowContext(ctx, query, organisationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}
