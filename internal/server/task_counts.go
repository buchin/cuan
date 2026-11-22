package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// taskCounts is a lightweight read model. Total includes canceled work for existing
// counters; completion progress uses Total-Canceled, and only accepted done work.
type taskCounts struct{ Total, Active, Queued, Done, Failed, Review, Canceled int }
type scopedTaskCounts struct{ Rooms, Projects map[string]taskCounts }

func (s *Server) invalidateTaskCounts() {
	s.taskCountsMu.Lock()
	s.taskCountsExpires = time.Time{}
	s.taskCountsMu.Unlock()
}

func (s *Server) readTaskCounts(ctx context.Context) (scopedTaskCounts, error) {
	if s.taskCountsTTL > 0 {
		now := time.Now()
		s.taskCountsMu.RLock()
		if now.Before(s.taskCountsExpires) {
			cached := s.taskCountsCached
			s.taskCountsMu.RUnlock()
			return cached, nil
		}
		s.taskCountsMu.RUnlock()
	}

	result, err := s.queryTaskCounts(ctx)
	if err != nil {
		return result, err
	}

	if s.taskCountsTTL > 0 {
		s.taskCountsMu.Lock()
		s.taskCountsCached = result
		s.taskCountsExpires = time.Now().Add(s.taskCountsTTL)
		s.taskCountsMu.Unlock()
	}
	return result, nil
}

// readProjectTaskProgress returns the task progress for a specific project.
// If valid cached task counts are present, it avoids any DB queries.
// Otherwise, it performs a targeted single-project aggregate query rather than a full DB scan.
func (s *Server) readProjectTaskProgress(ctx context.Context, projectID string) (*project.TaskProgress, error) {
	if projectID == "" {
		return &project.TaskProgress{}, nil
	}
	s.taskCountsMu.RLock()
	if time.Now().Before(s.taskCountsExpires) {
		if c, ok := s.taskCountsCached.Projects[projectID]; ok {
			s.taskCountsMu.RUnlock()
			return &project.TaskProgress{
				Total:    c.Total - c.Canceled,
				Done:     c.Done,
				Active:   c.Active,
				Queued:   c.Queued,
				Review:   c.Review,
				Blocked:  c.Failed,
				Canceled: c.Canceled,
			}, nil
		}
	}
	s.taskCountsMu.RUnlock()

	if s.db != nil {
		var total, active, queued, done, failed, review, canceled int
		err := s.db.QueryRowContext(ctx, singleProjectTaskCountsSQL, projectID).Scan(
			&total, &active, &queued, &done, &failed, &review, &canceled,
		)
		if err != nil {
			return nil, err
		}
		return &project.TaskProgress{
			Total:    total - canceled,
			Done:     done,
			Active:   active,
			Queued:   queued,
			Review:   review,
			Blocked:  failed,
			Canceled: canceled,
		}, nil
	}

	counts, err := s.readTaskCounts(ctx)
	if err != nil {
		return nil, err
	}
	c := counts.Projects[projectID]
	return &project.TaskProgress{
		Total:    c.Total - c.Canceled,
		Done:     c.Done,
		Active:   c.Active,
		Queued:   c.Queued,
		Review:   c.Review,
		Blocked:  c.Failed,
		Canceled: c.Canceled,
	}, nil
}

const singleProjectTaskCountsSQL = `
WITH project_task_ids AS (
 SELECT id FROM tasks WHERE project_id = $1
 UNION
 SELECT task_id AS id FROM task_projects WHERE project_id = $1
 UNION
 SELECT id::text AS id FROM project_tasks WHERE project_id = $1
),
latest_attempts AS (
 SELECT DISTINCT ON (a.task_id) a.id, a.task_id, a.status
 FROM task_attempts a
 JOIN project_task_ids pti ON pti.id = a.task_id
 ORDER BY a.task_id, a.created_at DESC, a.id DESC
),
latest_launches AS (
 SELECT DISTINCT ON (l.task_id) l.task_id, l.state
 FROM task_launches l
 JOIN project_task_ids pti ON pti.id = l.task_id
 LEFT JOIN latest_attempts a ON a.task_id = l.task_id AND a.id = l.attempt_id AND a.status IN ('queued','running')
 ORDER BY l.task_id, (a.id IS NOT NULL) DESC, l.created_at DESC, l.id DESC
),
canonical AS (
 SELECT t.id,
 ` + task.EffectiveStatusSQL + ` AS status
 FROM tasks t
 JOIN project_task_ids pti ON pti.id = t.id
 LEFT JOIN latest_attempts a ON a.task_id = t.id
 LEFT JOIN latest_launches l ON l.task_id = t.id
 UNION ALL
 SELECT p.id::text, lower(btrim(p.status)) FROM project_tasks p
 WHERE p.project_id = $1 AND NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = p.id::text)
)
SELECT COUNT(*),
 COUNT(*) FILTER (WHERE status IN ('doing','in_progress','running','working')),
 COUNT(*) FILTER (WHERE status IN ('queued','pending')),
 COUNT(*) FILTER (WHERE status = 'done'),
 COUNT(*) FILTER (WHERE status IN ('blocked','failed')),
 COUNT(*) FILTER (WHERE status = 'review'),
 COUNT(*) FILTER (WHERE status IN ('canceled','cancelled'))
FROM canonical`

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

func (s *Server) queryTaskCounts(ctx context.Context) (scopedTaskCounts, error) {
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
