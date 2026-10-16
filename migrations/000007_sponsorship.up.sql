-- Cuan Migration 000007: Sponsorship, Batches, Orders, Waiting List & Audit Logs
-- (000007_sponsorship.up.sql)

CREATE TABLE IF NOT EXISTS sponsorship_batches (
    id VARCHAR(64) PRIMARY KEY,
    batch_slug VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open', -- open, closed
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_sponsorship_batches_slug ON sponsorship_batches(batch_slug);

-- Seed initial batch v1.0 Founding Backers
INSERT INTO sponsorship_batches (id, batch_slug, name, status)
VALUES ('batch_v1_founding', 'v1.0', 'v1.0 Founding Backers', 'open')
ON CONFLICT (batch_slug) DO NOTHING;

CREATE TABLE IF NOT EXISTS sponsors (
    id VARCHAR(64) PRIMARY KEY,
    batch_id VARCHAR(64) REFERENCES sponsorship_batches(id) ON DELETE CASCADE,
    brand_name VARCHAR(255) NOT NULL,
    website_url TEXT NOT NULL,
    slogan VARCHAR(255),
    logo_path TEXT NOT NULL,
    contact_email VARCHAR(255) NOT NULL,
    contact_wa VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'pending_review', -- pending_review, approved, rejected
    rejection_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sponsors_batch_id ON sponsors(batch_id);
CREATE INDEX IF NOT EXISTS idx_sponsors_status ON sponsors(status);

CREATE TABLE IF NOT EXISTS sponsorship_orders (
    id VARCHAR(64) PRIMARY KEY,
    sponsor_id VARCHAR(64) REFERENCES sponsors(id) ON DELETE SET NULL,
    dojo_order_id VARCHAR(128) UNIQUE NOT NULL,
    amount_idr BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- pending, paid, failed
    paid_at TIMESTAMP WITH TIME ZONE,
    payload JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sponsorship_orders_dojo_order_id ON sponsorship_orders(dojo_order_id);
CREATE INDEX IF NOT EXISTS idx_sponsorship_orders_sponsor_id ON sponsorship_orders(sponsor_id);
CREATE INDEX IF NOT EXISTS idx_sponsorship_orders_status ON sponsorship_orders(status);

CREATE TABLE IF NOT EXISTS waiting_list_leads (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    source VARCHAR(100) DEFAULT 'landing_page',
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- pending, verified, converted
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_waiting_list_leads_email ON waiting_list_leads(email);

CREATE TABLE IF NOT EXISTS sponsorship_activities (
    id BIGSERIAL PRIMARY KEY,
    sponsor_id VARCHAR(64) REFERENCES sponsors(id) ON DELETE CASCADE,
    order_id VARCHAR(64) REFERENCES sponsorship_orders(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    details TEXT,
    actor VARCHAR(128) DEFAULT 'system',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sponsorship_activities_sponsor_id ON sponsorship_activities(sponsor_id);
