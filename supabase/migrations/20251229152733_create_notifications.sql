-- Notifications table for in-app and external channel delivery
CREATE TABLE notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- NULL = org-wide notification
  type TEXT NOT NULL,                                   -- job_complete, job_failed, scheduler_run, etc.
  title TEXT NOT NULL,
  message TEXT,
  data JSONB,                                           -- Structured payload (job_id, domain, stats, etc.)
  is_read BOOLEAN NOT NULL DEFAULT false,
  delivered_slack BOOLEAN NOT NULL DEFAULT false,       -- Track channel delivery status
  delivered_email BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_org ON notifications(organisation_id);
CREATE INDEX idx_notifications_user ON notifications(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_notifications_unread ON notifications(organisation_id, is_read) WHERE is_read = false;
CREATE INDEX idx_notifications_pending_slack ON notifications(delivered_slack) WHERE delivered_slack = false;
CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_created ON notifications(created_at DESC);

-- Enable RLS
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;

-- Users can view notifications for their org
CREATE POLICY "notifications_select_own_org" ON notifications
  FOR SELECT USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Users can mark their own notifications as read
CREATE POLICY "notifications_update_own" ON notifications
  FOR UPDATE USING (
    organisation_id IN (SELECT organisation_id FROM users WHERE id = auth.uid())
  );

-- Trigger to notify on new notification (for real-time processing)
CREATE OR REPLACE FUNCTION notify_new_notification()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM pg_notify('new_notification', json_build_object(
    'id', NEW.id,
    'organisation_id', NEW.organisation_id,
    'type', NEW.type
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notification_insert_trigger
  AFTER INSERT ON notifications
  FOR EACH ROW
  EXECUTE FUNCTION notify_new_notification();
