package task

import "strings"

// Initial metadata is local and deterministic; the bound agent can refine it.
func inferPrompt(prompt string) (string, string) {
	text := strings.TrimSpace(prompt)
	line := strings.SplitN(text, "\n", 2)[0]
	line = strings.TrimSpace(strings.TrimLeft(line, "#* -\t"))
	if line == "" {
		line = text
	}
	runes := []rune(line)
	if len(runes) > 110 {
		line = string(runes[:107]) + "…"
	}
	priority := PriorityNormal
	words := " " + strings.ToLower(text) + " "
	for _, signal := range []string{"production down", "data loss", "security breach", "critical outage"} {
		if strings.Contains(words, signal) {
			return line, PriorityUrgent
		}
	}
	for _, signal := range []string{" urgent ", " asap ", "high priority", "blocking users"} {
		if strings.Contains(words, signal) {
			priority = PriorityHigh
		}
	}
	return line, priority
}

// NormalizeExecutionCategory cleans and validates an execution category, returning "code", "ops", or "readonly".
// Returns empty string if the input is not a recognized category.
func NormalizeExecutionCategory(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case ExecutionCategoryCode:
		return ExecutionCategoryCode
	case ExecutionCategoryOps:
		return ExecutionCategoryOps
	case ExecutionCategoryReadOnly, "read_only", "read-only":
		return ExecutionCategoryReadOnly
	default:
		return ""
	}
}

// InferExecutionCategory determines the appropriate execution lane (code, ops, readonly)
// based on task title and instructions.
func InferExecutionCategory(title, instructions string) string {
	combined := strings.ToLower(strings.TrimSpace(title + " " + instructions))

	// Database schema migrations always belong in code lane due to exclusive DDL locking requirements
	if IsSchemaMigrationPrompt(title, instructions) {
		return ExecutionCategoryCode
	}

	// Code creation/modification signals take precedence over incidental words like 'view'
	codeSignals := []string{
		"integrasi", "implementasi", "tambahkan", "buat", "perbaiki", "ubah",
		"integrate", "implementation", "feature", "component", "komponen", "renderer",
		"refactor", "bugfix", "fix ", "frontend", "backend", "deploy",
	}
	for _, s := range codeSignals {
		if strings.Contains(combined, s) {
			return ExecutionCategoryCode
		}
	}

	// Read-only signals: analysis, audit, inspection, monitoring, queries
	readonlySignals := []string{
		"audit", "analisis", "analysis", "inspect", "check status", "review code",
		"show ", "list ", "status check", "read-only", "readonly",
		"query ", "telemetry", "metrics", "log check", "monitor", "view only", "view logs",
	}
	for _, s := range readonlySignals {
		if strings.Contains(combined, s) {
			return ExecutionCategoryReadOnly
		}
	}

	// Ops signals: operational tasks like grant access, artisan commands, data/contact sync, maintenance
	opsSignals := []string{
		"grant access", "grant permission", "revoke permission", "artisan",
		"php artisan", "command artisan", "sync contact", "contact sync",
		"data sync", "sinkronisasi", "clear cache", "cache:clear", "route:clear",
		"config:clear", "seed database", "db:seed", "backup data", "maintenance",
		"operational", "ops task",
	}
	for _, s := range opsSignals {
		if strings.Contains(combined, s) {
			return ExecutionCategoryOps
		}
	}

	return ExecutionCategoryCode
}

// IsSchemaMigrationPrompt checks whether task instructions or title involve database schema migrations.
func IsSchemaMigrationPrompt(title, instructions string) bool {
	combined := strings.ToLower(strings.TrimSpace(title + " " + instructions))
	signals := []string{
		"migration", "migrate", "db:migrate", "artisan migrate",
		"alter table", "schema migration", "schema change",
		"add column", "drop column", "create table",
	}
	for _, s := range signals {
		if strings.Contains(combined, s) {
			return true
		}
	}
	return false
}

// IsTaskSchemaMigration checks if a task is a schema migration task.
// Only tasks in the code lane (or uncategorized) can be schema migrations; ops and readonly tasks never alter DDL schema.
func IsTaskSchemaMigration(t *Task) bool {
	if t == nil {
		return false
	}
	cat := NormalizeExecutionCategory(t.ExecutionCategory)
	if cat != "" && cat != ExecutionCategoryCode {
		return false
	}
	return IsSchemaMigrationPrompt(t.Title, t.Instructions)
}
