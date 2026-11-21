package server

import (
	"context"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// taskCounts is a lightweight read model. Total includes canceled work for existing
// counters; completion progress uses Total-Canceled, and only accepted done work.
type taskCounts struct{ Total, Active, Queued, Done, Failed, Review, Canceled int }
type scopedTaskCounts struct{ Rooms, Projects map[string]taskCounts }

// One grouped query, independent of task-list pagination. Never reads instructions,
// outputs or logs. Only the latest attempt/launch status is read; modern records
// supersede mirrored old rows, including canceled work saved in review.
// Computed via set-based aggregation CTEs instead of N+1 lateral loops per task.
const taskCountsSQL = `
WITH latest_attempts AS (
 SELECT DISTINCT ON (task_id) id, task_id, status
 FROM task_attempts
 ORDER BY task_id, created_at DESC, id DESC
),
latest_launches AS (
 SELECT DISTINCT ON (l.task_id) l.task_id, l.state
 FROM task_launches l
 LEFT JOIN latest_attempts a ON a.task_id = l.task_id AND a.id = l.attempt_id AND a.status IN ('queued','running')
 ORDER BY l.task_id, (a.id IS NOT NULL) DESC, l.created_at DESC, l.id DESC
),
canonical AS (
 SELECT t.id, t.project_id, t.source_room_id::text AS room_id,
 ` + task.EffectiveStatusSQL + ` AS status
 FROM tasks t
 LEFT JOIN latest_attempts a ON a.task_id = t.id
 LEFT JOIN latest_launches l ON l.task_id = t.id
 UNION ALL
 SELECT p.id::text, p.project_id, p.room_id::text, lower(btrim(p.status)) FROM project_tasks p
 WHERE NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = p.id::text)
), scopes AS (
 SELECT 'room' AS scope, room_id AS scope_id, id, status FROM canonical WHERE COALESCE(room_id,'') <> ''
 UNION
 SELECT 'project', project_id, id, status FROM canonical WHERE COALESCE(project_id,'') <> ''
 UNION
 SELECT 'project', p.project_id, c.id, c.status FROM canonical c JOIN task_projects p ON p.task_id = c.id
)
SELECT scope, scope_id, COUNT(*),
 COUNT(*) FILTER (WHERE status IN ('doing','in_progress','running','working')),
 COUNT(*) FILTER (WHERE status IN ('queued','pending')),
 COUNT(*) FILTER (WHERE status = 'done'),
 COUNT(*) FILTER (WHERE status IN ('blocked','failed')),
 COUNT(*) FILTER (WHERE status = 'review'),
 COUNT(*) FILTER (WHERE status IN ('canceled','cancelled'))
FROM scopes GROUP BY scope, scope_id`

func (s *Server) readTaskCounts(ctx context.Context) (scopedTaskCounts, error) {
	result := scopedTaskCounts{Rooms: map[string]taskCounts{}, Projects: map[string]taskCounts{}}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, taskCountsSQL)
		if err != nil {
			return result, err
		}
		defer rows.Close()
		for rows.Next() {
			var scope, id string
			var count taskCounts
			if err := rows.Scan(&scope, &id, &count.Total, &count.Active, &count.Queued, &count.Done, &count.Failed, &count.Review, &count.Canceled); err != nil {
				return result, err
			}
			if scope == "room" {
				result.Rooms[id] = count
			} else {
				result.Projects[id] = count
			}
		}
		return result, rows.Err()
	}
	// In-memory mode follows the same precedence, including old room/project tasks.
	type item struct {
		room, status string
		projects     []string
	}
	items := map[string]item{}
	if s.projectStudioRepo != nil {
		old, err := s.projectStudioRepo.ListTasks(ctx, db.ProjectTaskFilter{})
		if err != nil {
			return result, err
		}
		for _, t := range old {
			room := ""
			if t.RoomID != nil {
				room = *t.RoomID
			}
			items[t.ID] = item{room, t.Status, append(append([]string{}, t.ProjectIDs...), t.ProjectID)}
		}
	}
	if s.taskSvc != nil {
		tasks, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{})
		if err != nil {
			return result, err
		}
		launches := map[string]*task.LaunchInfo{}
		reader, batched := s.taskSvc.Store().(interface {
			LatestLaunchInfos(context.Context, []string) (map[string]*task.LaunchInfo, error)
		})
		if batched {
			ids := make([]string, 0, len(tasks))
			for _, t := range tasks {
				ids = append(ids, t.ID)
			}
			launches, err = reader.LatestLaunchInfos(ctx, ids)
			if err != nil {
				return result, err
			}
		}
		for _, t := range tasks {
			attempt, _ := s.taskSvc.Store().GetLatestAttempt(ctx, t.ID)
			launch := launches[t.ID]
			if !batched {
				if raw, _ := s.taskSvc.LatestLaunch(ctx, t.ID); raw != nil {
					launch = &task.LaunchInfo{State: raw.State}
				}
			}
			attemptStatus, launchState := "", ""
			if attempt != nil {
				attemptStatus = attempt.Status
			}
			if launch != nil {
				launchState = launch.State
			}
			items[t.ID] = item{t.SourceRoomID, taskCountStatus(t.Status, attemptStatus, launchState), append(append([]string{}, t.ProjectIDs...), t.ProjectID)}
		}
	}
	for _, t := range items {
		if t.room != "" {
			result.Rooms[t.room] = addTaskCount(result.Rooms[t.room], t.status)
		}
		seen := map[string]bool{}
		for _, id := range t.projects {
			if id != "" && !seen[id] {
				result.Projects[id] = addTaskCount(result.Projects[id], t.status)
				seen[id] = true
			}
		}
	}
	return result, nil
}

func addTaskCount(c taskCounts, status string) taskCounts {
	c.Total++
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done":
		c.Done++
	case "doing", "in_progress", "running", "working":
		c.Active++
	case "queued", "pending":
		c.Queued++
	case "blocked", "failed":
		c.Failed++
	case "review":
		c.Review++
	case "canceled", "cancelled":
		c.Canceled++
	}
	return c
}

// Presentation only: stopping a run does not overwrite its saved review/history.
func taskCountStatus(status, attempt, launch string) string {
	return task.EffectiveStatus(status, attempt, launch)
}
