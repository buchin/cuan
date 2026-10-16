-- Cuan Migration 000007 Rollback: Sponsorship, Batches, Orders, Waiting List & Audit Logs
-- (000007_sponsorship.down.sql)

DROP TABLE IF EXISTS sponsorship_activities CASCADE;
DROP TABLE IF EXISTS sponsorship_orders CASCADE;
DROP TABLE IF EXISTS sponsors CASCADE;
DROP TABLE IF EXISTS sponsorship_batches CASCADE;
DROP TABLE IF EXISTS waiting_list_leads CASCADE;
