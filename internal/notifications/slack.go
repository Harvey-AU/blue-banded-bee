package notifications

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
)

// Service handles notification creation and delivery
type Service struct {
	db       NotificationDB
	channels []DeliveryChannel
}

// NotificationDB defines the database operations needed by the service
type NotificationDB interface {
	CreateNotification(ctx context.Context, n *db.Notification) error
	GetPendingSlackNotifications(ctx context.Context, limit int) ([]*db.Notification, error)
	MarkNotificationDelivered(ctx context.Context, notificationID, channel string) error
	GetSlackConnectionsForOrg(ctx context.Context, organisationID string) ([]*db.SlackConnection, error)
	GetEnabledUserLinksForConnection(ctx context.Context, connectionID string) ([]*db.SlackUserLink, error)
}

// DeliveryChannel defines the interface for notification delivery
type DeliveryChannel interface {
	Name() string
	Deliver(ctx context.Context, n *db.Notification) error
}

// NewService creates a notification service
func NewService(database NotificationDB) *Service {
	return &Service{db: database}
}

// AddChannel adds a delivery channel to the service
func (s *Service) AddChannel(ch DeliveryChannel) {
	s.channels = append(s.channels, ch)
}

// CreateJobCompleteNotification creates a notification for a completed job
func (s *Service) CreateJobCompleteNotification(ctx context.Context, job *jobs.Job) error {
	if job.OrganisationID == nil {
		log.Debug().Str("job_id", job.ID).Msg("Job has no organisation, skipping notification")
		return nil
	}

	// Calculate duration
	duration := "N/A"
	if job.DurationSeconds != nil && *job.DurationSeconds > 0 {
		d := time.Duration(*job.DurationSeconds) * time.Second
		if d < time.Minute {
			duration = fmt.Sprintf("%ds", int(d.Seconds()))
		} else if d < time.Hour {
			duration = fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
		} else {
			duration = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		}
	}

	n := &db.Notification{
		ID:             uuid.New().String(),
		OrganisationID: *job.OrganisationID,
		UserID:         job.UserID, // Notify the user who created the job
		Type:           db.NotificationJobComplete,
		Title:          fmt.Sprintf("Cache warming complete: %s", job.Domain),
		Message:        fmt.Sprintf("%d pages warmed in %s", job.CompletedTasks, duration),
		Data: map[string]interface{}{
			"job_id":          job.ID,
			"domain":          job.Domain,
			"completed_tasks": job.CompletedTasks,
			"failed_tasks":    job.FailedTasks,
			"duration":        duration,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.db.CreateNotification(ctx, n); err != nil {
		return fmt.Errorf("failed to create job complete notification: %w", err)
	}

	log.Info().
		Str("notification_id", n.ID).
		Str("job_id", job.ID).
		Str("domain", job.Domain).
		Msg("Job complete notification created")

	return nil
}

// CreateJobFailedNotification creates a notification for a failed job
func (s *Service) CreateJobFailedNotification(ctx context.Context, job *jobs.Job) error {
	if job.OrganisationID == nil {
		return nil
	}

	n := &db.Notification{
		ID:             uuid.New().String(),
		OrganisationID: *job.OrganisationID,
		UserID:         job.UserID,
		Type:           db.NotificationJobFailed,
		Title:          fmt.Sprintf("Cache warming failed: %s", job.Domain),
		Message:        job.ErrorMessage,
		Data: map[string]interface{}{
			"job_id":        job.ID,
			"domain":        job.Domain,
			"error_message": job.ErrorMessage,
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := s.db.CreateNotification(ctx, n); err != nil {
		return fmt.Errorf("failed to create job failed notification: %w", err)
	}

	log.Info().
		Str("notification_id", n.ID).
		Str("job_id", job.ID).
		Str("domain", job.Domain).
		Msg("Job failed notification created")

	return nil
}

// NotifyJobComplete implements the JobNotifier interface for the worker pool
func (s *Service) NotifyJobComplete(ctx context.Context, job *jobs.Job) {
	if err := s.CreateJobCompleteNotification(ctx, job); err != nil {
		log.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to create job complete notification")
	}
}

// NotifyJobFailed implements the JobNotifier interface for the worker pool
func (s *Service) NotifyJobFailed(ctx context.Context, job *jobs.Job) {
	if err := s.CreateJobFailedNotification(ctx, job); err != nil {
		log.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to create job failed notification")
	}
}

// ProcessPendingNotifications delivers pending notifications to all channels
func (s *Service) ProcessPendingNotifications(ctx context.Context, limit int) error {
	for _, ch := range s.channels {
		if err := s.deliverToChannel(ctx, ch, limit); err != nil {
			log.Warn().Err(err).Str("channel", ch.Name()).Msg("Failed to deliver notifications")
		}
	}
	return nil
}

func (s *Service) deliverToChannel(ctx context.Context, ch DeliveryChannel, limit int) error {
	var notifications []*db.Notification
	var err error

	switch ch.Name() {
	case "slack":
		notifications, err = s.db.GetPendingSlackNotifications(ctx, limit)
	default:
		return nil // Unknown channel, skip
	}

	if err != nil {
		return err
	}

	for _, n := range notifications {
		if err := ch.Deliver(ctx, n); err != nil {
			log.Warn().
				Err(err).
				Str("notification_id", n.ID).
				Str("channel", ch.Name()).
				Msg("Failed to deliver notification")
			continue
		}

		if err := s.db.MarkNotificationDelivered(ctx, n.ID, ch.Name()); err != nil {
			log.Warn().
				Err(err).
				Str("notification_id", n.ID).
				Msg("Failed to mark notification delivered")
		}
	}

	return nil
}

// SlackChannel implements the DeliveryChannel interface for Slack
type SlackChannel struct {
	db SlackDB
}

// SlackDB defines Slack-specific database operations
type SlackDB interface {
	GetSlackConnectionsForOrg(ctx context.Context, organisationID string) ([]*db.SlackConnection, error)
	GetEnabledUserLinksForConnection(ctx context.Context, connectionID string) ([]*db.SlackUserLink, error)
	GetSlackToken(ctx context.Context, connectionID string) (string, error)
}

// NewSlackChannel creates a new Slack delivery channel
func NewSlackChannel(database SlackDB) (*SlackChannel, error) {
	return &SlackChannel{db: database}, nil
}

// Name returns the channel name
func (c *SlackChannel) Name() string {
	return "slack"
}

// Deliver sends a notification to Slack
func (c *SlackChannel) Deliver(ctx context.Context, n *db.Notification) error {
	connections, err := c.db.GetSlackConnectionsForOrg(ctx, n.OrganisationID)
	if err != nil {
		return fmt.Errorf("failed to fetch Slack connections: %w", err)
	}

	if len(connections) == 0 {
		return nil
	}

	var lastErr error
	for _, conn := range connections {
		if err := c.deliverToConnection(ctx, conn, n); err != nil {
			log.Warn().
				Err(err).
				Str("workspace_id", conn.WorkspaceID).
				Str("notification_id", n.ID).
				Msg("Failed to deliver to Slack workspace")
			lastErr = err
		}
	}
	return lastErr
}

func (c *SlackChannel) deliverToConnection(ctx context.Context, conn *db.SlackConnection, n *db.Notification) error {
	// Get token from Supabase Vault
	token, err := c.db.GetSlackToken(ctx, conn.ID)
	if err != nil {
		return fmt.Errorf("failed to get access token from vault: %w", err)
	}

	client := slack.New(token)

	links, err := c.db.GetEnabledUserLinksForConnection(ctx, conn.ID)
	if err != nil {
		return fmt.Errorf("failed to get user links: %w", err)
	}

	if len(links) == 0 {
		return nil
	}

	blocks := c.buildMessageBlocks(n)
	fallbackText := fmt.Sprintf("%s: %s", n.Title, n.Message)

	var lastErr error
	for _, link := range links {
		_, _, err := client.PostMessage(
			link.SlackUserID,
			slack.MsgOptionBlocks(blocks...),
			slack.MsgOptionText(fallbackText, false),
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("slack_user_id", link.SlackUserID).
				Str("notification_id", n.ID).
				Msg("Failed to send Slack DM")
			lastErr = err
		} else {
			log.Info().
				Str("slack_user_id", link.SlackUserID).
				Str("notification_id", n.ID).
				Str("workspace_name", conn.WorkspaceName).
				Msg("Slack DM sent")
		}
	}

	return lastErr
}

func (c *SlackChannel) buildMessageBlocks(n *db.Notification) []slack.Block {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://app.bluebandedbee.co"
	}

	var emoji string
	switch n.Type {
	case db.NotificationJobComplete:
		emoji = ":white_check_mark:"
	case db.NotificationJobFailed:
		emoji = ":x:"
	default:
		emoji = ":bell:"
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("%s *%s*", emoji, n.Title),
				false,
				false,
			),
			nil,
			nil,
		),
	}

	// Add message if present
	if n.Message != "" {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", n.Message, false, false),
			nil,
			nil,
		))
	}

	// Add link to job if job_id is in data
	if jobID, ok := n.Data["job_id"].(string); ok && jobID != "" {
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("<%s/jobs/%s|View details>", appURL, jobID),
				false,
				false,
			),
			nil,
			nil,
		))
	}

	return blocks
}
