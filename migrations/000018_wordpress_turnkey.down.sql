-- Cuan Migration 000018: WordPress Turnkey Cloud & Webmaster Data Layer (Rollback)
-- (000018_wordpress_turnkey.down.sql)

DROP INDEX IF EXISTS idx_wordpress_webmaster_actions_applied_at;
DROP INDEX IF EXISTS idx_wordpress_webmaster_actions_action_type;
DROP INDEX IF EXISTS idx_wordpress_webmaster_actions_deployment_id;
DROP TABLE IF EXISTS wordpress_webmaster_actions CASCADE;

DROP INDEX IF EXISTS idx_wordpress_plugins_themes_slug;
DROP INDEX IF EXISTS idx_wordpress_plugins_themes_kind;
DROP INDEX IF EXISTS idx_wordpress_plugins_themes_deployment_id;
DROP TABLE IF EXISTS wordpress_plugins_themes CASCADE;

DROP INDEX IF EXISTS idx_wordpress_deployments_health_status;
DROP INDEX IF EXISTS idx_wordpress_deployments_status;
DROP INDEX IF EXISTS idx_wordpress_deployments_domain;
DROP INDEX IF EXISTS idx_wordpress_deployments_project_id;
DROP TABLE IF EXISTS wordpress_deployments CASCADE;
