# Slack Integration Plan

## Overview

Enable Blue Banded Bee organisations to connect with Slack for job
notifications. Three-phase approach:

1. **Connect Org to Slack Workspace** - OAuth installation
2. **Personal Notifications** - DMs to linked users
3. **Channel Notifications** - Post to team channels

---

## Step 1: Connect Org to Slack Workspace

### Slack App Setup

1. Create Slack App at api.slack.com
2. Configure OAuth scopes:
   - `chat:write` - Send messages
   - `users:read` - List workspace users for linking
   - `channels:read` - List channels for subscriptions
   - `incoming-webhook` - Optional fallback
3. Set redirect URL: `https://app.bluebandedbee.co/integrations/slack/callback`

### Database Schema

```sql
CREATE TABLE slack_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID REFERENCES organisations(id) NOT NULL,
  workspace_id TEXT NOT NULL,           -- Slack team_id
  workspace_name TEXT,
  access_token TEXT NOT NULL,           -- encrypted
  bot_user_id TEXT,
  installing_user_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(organisation_id, workspace_id)
);
```

### OAuth Flow

1. User clicks "Connect Slack" in BBB dashboard
2. Redirect to Slack OAuth:
   ```
   https://slack.com/oauth/v2/authorize?
     client_id=XXX&
     scope=chat:write,users:read,channels:read&
     redirect_uri=https://app.bluebandedbee.co/integrations/slack/callback&
     state={org_id}
   ```
3. User authorises in Slack
4. Slack redirects back with `code`
5. Exchange code for access token via `oauth.v2.access`
6. Store connection linked to organisation

---

## Step 2: Personal Notifications (DMs to User)

### Database Schema

```sql
CREATE TABLE slack_user_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) NOT NULL,
  slack_connection_id UUID REFERENCES slack_connections(id) NOT NULL,
  slack_user_id TEXT NOT NULL,          -- Slack member ID
  dm_notifications BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, slack_connection_id)
);
```

### Link Flow

1. After org connects, user clicks "Link my Slack account"
2. Options:
   - **Email match**: Call `users.list`, match by email
   - **Identity scope**: Add `identity.basic` scope for direct link
3. Store `slack_user_id` for the BBB user

### Send DM

```go
// Use chat.postMessage with channel = slack_user_id (opens DM automatically)
_, _, err := slackClient.PostMessage(
  userSlackID,
  slack.MsgOptionText("Your job completed: example.com - 150 pages warmed", false),
)
```

---

## Step 3: Channel Notifications

### Database Schema

```sql
CREATE TABLE slack_channel_subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slack_connection_id UUID REFERENCES slack_connections(id) NOT NULL,
  channel_id TEXT NOT NULL,
  channel_name TEXT,
  notify_job_complete BOOLEAN DEFAULT true,
  notify_job_failed BOOLEAN DEFAULT true,
  notify_scheduler_run BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(slack_connection_id, channel_id)
);
```

### Channel Setup Flow

1. User selects channel from dropdown (fetch via `conversations.list`)
2. Bot must be invited to channel (or use `conversations.join` if public)
3. Store subscription preferences per channel

### Send to Channel

```go
slackClient.PostMessage(channelID, slack.MsgOptionBlocks(
  slack.NewSectionBlock(
    slack.NewTextBlockObject("mrkdwn",
      "*Job Completed*\n"+
      "Domain: example.com\n"+
      "Pages warmed: 150\n"+
      "<https://app.bluebandedbee.co/jobs/abc123|View Details>",
      false, false),
    nil, nil,
  ),
))
```

---

## API Endpoints

| Method | Endpoint                               | Description                  |
| ------ | -------------------------------------- | ---------------------------- |
| POST   | `/v1/integrations/slack/connect`       | Start OAuth flow             |
| GET    | `/v1/integrations/slack/callback`      | OAuth callback handler       |
| GET    | `/v1/integrations/slack`               | List org's Slack connections |
| DELETE | `/v1/integrations/slack/:id`           | Disconnect workspace         |
| POST   | `/v1/integrations/slack/link-user`     | Link BBB user to Slack user  |
| DELETE | `/v1/integrations/slack/link-user`     | Unlink Slack user            |
| GET    | `/v1/integrations/slack/channels`      | List available channels      |
| POST   | `/v1/integrations/slack/subscribe`     | Subscribe channel to notifs  |
| PUT    | `/v1/integrations/slack/subscribe/:id` | Update subscription prefs    |
| DELETE | `/v1/integrations/slack/subscribe/:id` | Unsubscribe channel          |

---

## Notification Triggers

Hook into existing job completion logic in `internal/jobs/manager.go`:

```go
// In job completion handler
func (m *Manager) completeJob(ctx context.Context, job *Job) error {
  // ... existing completion logic ...

  // Send Slack notifications
  if err := m.notifySlack(ctx, job); err != nil {
    log.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to send Slack notification")
    // Don't fail job completion for notification errors
  }

  return nil
}

func (m *Manager) notifySlack(ctx context.Context, job *Job) error {
  // 1. Get org's Slack connections
  // 2. For each connection:
  //    a. Check user DM preferences, send if enabled
  //    b. Check channel subscriptions, send to matching channels
  return nil
}
```

---

## Notification Types

| Event             | DM Default | Channel Default | Message Content                        |
| ----------------- | ---------- | --------------- | -------------------------------------- |
| Job completed     | Yes        | Yes             | Domain, pages warmed, duration, link   |
| Job failed        | Yes        | Yes             | Domain, error summary, link            |
| Scheduler started | No         | Optional        | Scheduled job started for domain       |
| Scheduler failed  | Yes        | Yes             | Scheduler couldn't create job + reason |

---

## Implementation Order

1. **Phase 1**: Slack App + OAuth connection flow
2. **Phase 2**: Basic job completion notifications to installing user
3. **Phase 3**: User linking for multi-user orgs
4. **Phase 4**: Channel subscriptions with preferences UI

---

## Security Considerations

- Encrypt `access_token` at rest
- Validate `state` parameter in OAuth callback to prevent CSRF
- Use Slack's token rotation if available
- Scope tokens to minimum required permissions
- Handle token revocation gracefully (Slack sends events)

---

## Dependencies

- Go Slack library: `github.com/slack-go/slack`
- Migration for new tables
- Dashboard UI for connection management
