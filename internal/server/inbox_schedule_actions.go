package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/scheduler"
)

type inboxScheduleAction struct {
	Action       string `json:"action"` // "create", "update", "delete", "toggle"
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	ScheduleType string `json:"schedule_type,omitempty"` // "daily", "weekly", "interval"
	TimeOfDay    string `json:"time_of_day,omitempty"`   // "08:00"
	DayOfWeek    int    `json:"day_of_week,omitempty"`   // 0=Sunday..6=Saturday
	IntervalMins int    `json:"interval_mins,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

func parseInboxSchedule(content string) (string, *inboxScheduleAction, error) {
	const marker = "```cuan-schedule\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid schedule action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxScheduleAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	act := strings.ToLower(strings.TrimSpace(a.Action))
	if act == "" {
		act = "create"
	}
	a.Action = act
	if a.Action == "create" && strings.TrimSpace(a.Title) == "" {
		return "", nil, errors.New("Schedule title is required.")
	}
	if (a.Action == "update" || a.Action == "delete" || a.Action == "toggle") && strings.TrimSpace(a.ID) == "" {
		return "", nil, errors.New("Schedule ID is required.")
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxSchedule(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxSchedule(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.taskManager == nil {
		return cleanText + "\n\n⚠️ Scheduler is unavailable: task manager is not configured."
	}

	switch action.Action {
	case "create":
		schedType := scheduler.ScheduleDaily
		if action.ScheduleType != "" {
			schedType = scheduler.ScheduleType(action.ScheduleType)
		}
		timeOfDay := action.TimeOfDay
		if timeOfDay == "" && schedType != scheduler.ScheduleInterval {
			timeOfDay = "08:00"
		}
		intervalMins := action.IntervalMins
		if intervalMins <= 0 && schedType == scheduler.ScheduleInterval {
			intervalMins = 60
		}
		task := &scheduler.ScheduledTask{
			Title:        strings.TrimSpace(action.Title),
			Prompt:       strings.TrimSpace(action.Prompt),
			ScheduleType: schedType,
			TimeOfDay:    timeOfDay,
			DayOfWeek:    action.DayOfWeek,
			IntervalMins: intervalMins,
			ProjectID:    strings.TrimSpace(action.ProjectID),
		}
		created, err := s.taskManager.CreateTask(task)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to create schedule: %v", err)
		}
		timing := string(created.ScheduleType)
		if created.TimeOfDay != "" {
			timing += " @ " + created.TimeOfDay + " WIB"
		}
		projInfo := ""
		if created.ProjectID != "" {
			projInfo = fmt.Sprintf(" | Project: `%s`", created.ProjectID)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Jadwal Rutin Aktif:** `%s` (%s)%s", created.Title, timing, projInfo)

	case "update":
		task := &scheduler.ScheduledTask{
			Title:        strings.TrimSpace(action.Title),
			Prompt:       strings.TrimSpace(action.Prompt),
			ScheduleType: scheduler.ScheduleType(action.ScheduleType),
			TimeOfDay:    strings.TrimSpace(action.TimeOfDay),
			DayOfWeek:    action.DayOfWeek,
			IntervalMins: action.IntervalMins,
			ProjectID:    strings.TrimSpace(action.ProjectID),
		}
		updated, err := s.taskManager.UpdateTask(action.ID, task)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to update schedule %s: %v", action.ID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Jadwal Rutin Diperbarui:** `%s` (%s)", updated.Title, updated.ID)

	case "delete":
		if err := s.taskManager.DeleteTask(action.ID); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to delete schedule %s: %v", action.ID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Jadwal Rutin Dihapus:** `%s`", action.ID)

	case "toggle":
		toggled, err := s.taskManager.ToggleTask(action.ID, action.Enabled)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to toggle schedule %s: %v", action.ID, err)
		}
		state := "diaktifkan"
		if !toggled.Enabled {
			state = "dijeda"
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Jadwal Rutin `%s` telah %s.**", toggled.Title, state)

	default:
		return cleanText
	}
}
