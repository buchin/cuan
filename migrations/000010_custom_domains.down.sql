-- Cuan Migration 000010 Rollback: Custom Domains & On-Demand TLS
-- (000010_custom_domains.down.sql)

DROP INDEX IF EXISTS idx_custom_domains_ssl_status;
DROP INDEX IF EXISTS idx_custom_domains_dns_status;
DROP INDEX IF EXISTS idx_custom_domains_domain;
DROP INDEX IF EXISTS idx_custom_domains_deployment_id;
DROP TABLE IF EXISTS custom_domains CASCADE;
