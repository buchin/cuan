-- Cuan Migration 000010: Custom Domains & On-Demand TLS
-- (000010_custom_domains.up.sql)

CREATE TABLE IF NOT EXISTS custom_domains (
    id VARCHAR(64) PRIMARY KEY,
    deployment_id VARCHAR(64) NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL UNIQUE,
    dns_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    ssl_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    verification_token VARCHAR(64),
    last_checked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_custom_domains_deployment_id ON custom_domains(deployment_id);
CREATE INDEX IF NOT EXISTS idx_custom_domains_domain ON custom_domains(domain);
CREATE INDEX IF NOT EXISTS idx_custom_domains_dns_status ON custom_domains(dns_status);
CREATE INDEX IF NOT EXISTS idx_custom_domains_ssl_status ON custom_domains(ssl_status);
