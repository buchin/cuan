-- Cuan Migration 000029: Agent Model Configurations (000029_agent_model_configs.down.sql)

ALTER TABLE agents
    DROP COLUMN IF EXISTS fallback_model_id,
    DROP COLUMN IF EXISTS temperature,
    DROP COLUMN IF EXISTS token_budget;
