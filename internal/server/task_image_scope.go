package server

import (
	"encoding/json"

	"github.com/buchin/cuan/internal/task"
)

// Delivered outputs remain review evidence, not owner inputs granting a later
// execution visual scope. An owner who supplies the image in the original
// message is still recognized by the separate source-message attachment check.
func taskDeliveredAttachmentIDs(d *task.TaskDetailResponse) map[string]bool {
	ids := map[string]bool{}
	if d == nil {
		return ids
	}
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || event.ActivityType != "work_files" || event.Author != "Cuan" {
			continue
		}
		var delivery struct {
			Attachments []struct {
				ID string `json:"id"`
			} `json:"attachments"`
		}
		if json.Unmarshal([]byte(event.Message), &delivery) != nil {
			continue
		}
		for _, a := range delivery.Attachments {
			if a.ID != "" {
				ids[a.ID] = true
			}
		}
	}
	return ids
}
