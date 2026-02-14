-- Migration: Add Paddle billing integration schema
--
-- Purpose:
-- 1. Map internal plans to Paddle price IDs
-- 2. Store organisation-level Paddle customer/subscription state
-- 3. Persist invoice history and webhook idempotency records

ALTER TABLE plans
ADD COLUMN IF NOT EXISTS paddle_price_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_plans_paddle_price_id
ON plans (paddle_price_id)
WHERE paddle_price_id IS NOT NULL;

ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS paddle_customer_id TEXT;

ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS paddle_subscription_id TEXT;

ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS subscription_status TEXT NOT NULL DEFAULT 'inactive';

ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS current_period_ends_at TIMESTAMPTZ;

ALTER TABLE organisations
ADD COLUMN IF NOT EXISTS paddle_updated_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_organisations_paddle_subscription_id
ON organisations (paddle_subscription_id)
WHERE paddle_subscription_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_organisations_paddle_customer_id
ON organisations (paddle_customer_id)
WHERE paddle_customer_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS billing_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    paddle_transaction_id TEXT NOT NULL UNIQUE,
    paddle_invoice_id TEXT,
    invoice_number TEXT,
    status TEXT NOT NULL,
    currency_code TEXT,
    total_amount_cents INTEGER DEFAULT 0,
    billed_at TIMESTAMPTZ,
    invoice_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_invoices_org_created
ON billing_invoices (organisation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS paddle_webhook_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'processing',
    error_message TEXT
);
