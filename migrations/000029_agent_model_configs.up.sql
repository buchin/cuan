-- Cuan Migration 000029: Agent Model Configurations (000029_agent_model_configs.up.sql)
-- Adds fallback_model_id, temperature, and token_budget columns to agents table

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS fallback_model_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS temperature DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS token_budget INTEGER;

-- Ensure default builtin agent profiles exist with model configuration
INSERT INTO agents (id, name, role, system_prompt, model_id, fallback_model_id, temperature, token_budget, provider, status, created_at, updated_at)
VALUES
    ('agent_sm', '@sm', 'Scrum Master (Orchestrator)', 'You coordinate Cuan conversations as Scrum Master. All agents in Inbox act as Scrum Master / Technical Orchestrator with Full MCP access across all projects and platform modules.', 'gemini-1.5-pro', 'gemini-1.5-flash', 0.7, 8192, 'google', 'active', NOW(), NOW()),
    ('agent_dev', '@dev', 'Development Team (Worker)', 'Autonomous development agent specialized in code implementation, test verification, bug fixing, and task execution.', 'gemini-1.5-pro', 'gemini-1.5-flash', 0.2, 8192, 'google', 'active', NOW(), NOW()),
    ('agent_support', '@support', 'Customer Support', 'Customer support specialist dedicated to answering customer questions, triaging issues, and maintaining helpful and friendly communication.', 'gemini-1.5-flash', 'gemini-1.5-pro', 0.5, 4096, 'google', 'active', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    fallback_model_id = CASE WHEN agents.fallback_model_id = '' THEN EXCLUDED.fallback_model_id ELSE agents.fallback_model_id END,
    temperature = COALESCE(agents.temperature, EXCLUDED.temperature),
    token_budget = COALESCE(agents.token_budget, EXCLUDED.token_budget);
