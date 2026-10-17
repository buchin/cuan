-- Cuan Migration 000018: WordPress Turnkey Cloud & Webmaster Data Layer
-- (000018_wordpress_turnkey.up.sql)

CREATE TABLE IF NOT EXISTS wordpress_deployments (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    site_title VARCHAR(255) NOT NULL,
    admin_email VARCHAR(255) NOT NULL,
    admin_username VARCHAR(100) NOT NULL,
    admin_password_encrypted TEXT NOT NULL,
    db_name VARCHAR(100) NOT NULL,
    db_user VARCHAR(100) NOT NULL,
    db_password_encrypted TEXT NOT NULL,
    db_host VARCHAR(255) NOT NULL DEFAULT '127.0.0.1',
    db_port INTEGER NOT NULL DEFAULT 3306,
    domain VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    table_prefix VARCHAR(20) NOT NULL DEFAULT 'wp_',
    wp_version VARCHAR(50) NOT NULL DEFAULT 'latest',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    health_status VARCHAR(50) NOT NULL DEFAULT 'unknown',
    permalink_structure VARCHAR(255) NOT NULL DEFAULT '/%postname%/',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wordpress_deployments_project_id ON wordpress_deployments(project_id);
CREATE INDEX IF NOT EXISTS idx_wordpress_deployments_domain ON wordpress_deployments(domain);
CREATE INDEX IF NOT EXISTS idx_wordpress_deployments_status ON wordpress_deployments(status);
CREATE INDEX IF NOT EXISTS idx_wordpress_deployments_health_status ON wordpress_deployments(health_status);

CREATE TABLE IF NOT EXISTS wordpress_plugins_themes (
    id VARCHAR(64) PRIMARY KEY,
    deployment_id VARCHAR(64) NOT NULL REFERENCES wordpress_deployments(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    source_zip VARCHAR(255),
    installed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wordpress_plugins_themes_deployment_id ON wordpress_plugins_themes(deployment_id);
CREATE INDEX IF NOT EXISTS idx_wordpress_plugins_themes_kind ON wordpress_plugins_themes(kind);
CREATE INDEX IF NOT EXISTS idx_wordpress_plugins_themes_slug ON wordpress_plugins_themes(deployment_id, kind, slug);

CREATE TABLE IF NOT EXISTS wordpress_webmaster_actions (
    id VARCHAR(64) PRIMARY KEY,
    deployment_id VARCHAR(64) NOT NULL REFERENCES wordpress_deployments(id) ON DELETE CASCADE,
    action_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wordpress_webmaster_actions_deployment_id ON wordpress_webmaster_actions(deployment_id);
CREATE INDEX IF NOT EXISTS idx_wordpress_webmaster_actions_action_type ON wordpress_webmaster_actions(action_type);
CREATE INDEX IF NOT EXISTS idx_wordpress_webmaster_actions_applied_at ON wordpress_webmaster_actions(applied_at);
