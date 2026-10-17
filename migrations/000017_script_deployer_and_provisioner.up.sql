-- Cuan Migration 000017: Script Deployer & Database Provisioner
-- (000017_script_deployer_and_provisioner.up.sql)

CREATE TABLE IF NOT EXISTS script_deployments (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    server_id VARCHAR(64) REFERENCES servers(id) ON DELETE SET NULL,
    port INTEGER NOT NULL DEFAULT 0,
    domain VARCHAR(255) NOT NULL DEFAULT '',
    env_encrypted TEXT NOT NULL DEFAULT '',
    build_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    health VARCHAR(32) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_script_deployments_project_id ON script_deployments(project_id);
CREATE INDEX IF NOT EXISTS idx_script_deployments_server_id ON script_deployments(server_id);
CREATE INDEX IF NOT EXISTS idx_script_deployments_build_status ON script_deployments(build_status);
CREATE INDEX IF NOT EXISTS idx_script_deployments_health ON script_deployments(health);
CREATE INDEX IF NOT EXISTS idx_script_deployments_domain ON script_deployments(domain);

CREATE TABLE IF NOT EXISTS provisioned_databases (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    server_id VARCHAR(64) REFERENCES servers(id) ON DELETE SET NULL,
    engine VARCHAR(32) NOT NULL DEFAULT 'sqlite',
    db_name VARCHAR(128) NOT NULL,
    db_user VARCHAR(128) NOT NULL DEFAULT '',
    db_password_encrypted TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'provisioning',
    schema_imported BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provisioned_databases_project_id ON provisioned_databases(project_id);
CREATE INDEX IF NOT EXISTS idx_provisioned_databases_server_id ON provisioned_databases(server_id);
CREATE INDEX IF NOT EXISTS idx_provisioned_databases_engine ON provisioned_databases(engine);
CREATE INDEX IF NOT EXISTS idx_provisioned_databases_status ON provisioned_databases(status);
