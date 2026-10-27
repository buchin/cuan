package server

import (
	"context"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/task"
)

type UniversalSearchItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}
type UniversalSearchGroup struct {
	Label   string                `json:"label,omitempty"`
	Kind    string                `json:"kind"`
	Items   []UniversalSearchItem `json:"items"`
	HasMore bool                  `json:"has_more"`
	Error   string                `json:"error,omitempty"`
}

func searchPage(items []UniversalSearchItem, offset int) []UniversalSearchItem {
	if offset >= len(items) {
		return []UniversalSearchItem{}
	}
	end := offset + 6
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
func contactSearchTitle(name, email string) string {
	if name != "" {
		return name
	}
	if email != "" {
		return email
	}
	return "Anonymous visitor"
}
func (s *Server) globalNotes(ctx context.Context, q string, offset int) ([]UniversalSearchItem, error) {
	out := []UniversalSearchItem{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT m.id,c.id,c.name,c.email,m.body,m.created_at FROM contact_memory m JOIN crm_contacts c ON c.id=m.contact_id WHERE m.deleted_at IS NULL AND strpos(lower(m.body),$1)>0 ORDER BY m.created_at DESC,m.id DESC LIMIT 6 OFFSET $2`, q, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, contact, name, email, body string
			var at time.Time
			if err = rows.Scan(&id, &contact, &name, &email, &body, &at); err != nil {
				return nil, err
			}
			out = append(out, UniversalSearchItem{ID: id, Title: contactSearchTitle(name, email), Snippet: memorySnippet(body, q), URL: "/contacts?" + url.Values{"contact": {contact}, "note": {id}, "note_query": {q}}.Encode()})
		}
		return out, rows.Err()
	}
	s.crm.Lock()
	byID := map[string]CRMContact{}
	for _, c := range s.crm.contacts {
		byID[c.ID] = c
	}
	entries := []ContactMemory{}
	for _, m := range s.crm.memory {
		if m.DeletedAt == nil && strings.Contains(strings.ToLower(m.Body), q) {
			entries = append(entries, m)
		}
	}
	s.crm.Unlock()
	sortMemory(entries)
	for _, m := range entries {
		c, ok := byID[m.ContactID]
		if !ok {
			continue
		}
		out = append(out, UniversalSearchItem{ID: m.ID, Title: contactSearchTitle(c.Name, c.Email), Snippet: memorySnippet(m.Body, q), URL: "/contacts?" + url.Values{"contact": {c.ID}, "note": {m.ID}, "note_query": {q}}.Encode()})
	}
	return searchPage(out, offset), nil
}
func (s *Server) globalProjects(ctx context.Context, q string, offset int) ([]UniversalSearchItem, error) {
	out := []UniversalSearchItem{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT id,name,COALESCE(repo_url,''),is_archived FROM projects WHERE strpos(lower(name||' '||COALESCE(repo_url,'')),$1)>0 ORDER BY CASE WHEN strpos(lower(name),$1)>0 THEN 0 ELSE 1 END,updated_at DESC,id LIMIT 6 OFFSET $2`, q, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, repo string
			var archived bool
			if err = rows.Scan(&id, &name, &repo, &archived); err != nil {
				return nil, err
			}
			if archived {
				repo = "Archived · " + repo
			}
			out = append(out, UniversalSearchItem{ID: id, Title: name, Snippet: repo, URL: "/projects/" + url.PathEscape(id) + "/tasks"})
		}
		return out, rows.Err()
	}
	if s.projectSvc == nil {
		return out, nil
	}
	projects, err := s.projectSvc.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(projects, func(i, j int) bool {
		a, b := strings.Contains(strings.ToLower(projects[i].Name), q), strings.Contains(strings.ToLower(projects[j].Name), q)
		if a != b {
			return a
		}
		return projects[i].UpdatedAt.After(projects[j].UpdatedAt)
	})
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name+" "+p.RepoURL), q) {
			snippet := p.RepoURL
			if p.IsArchived {
				snippet = "Archived · " + snippet
			}
			out = append(out, UniversalSearchItem{ID: p.ID, Title: p.Name, Snippet: snippet, URL: "/projects/" + url.PathEscape(p.ID) + "/tasks"})
		}
	}
	return searchPage(out, offset), nil
}
func taskSearchURL(project, id string) string {
	if project == "" {
		return "/tasks/" + url.PathEscape(id)
	}
	return "/projects/" + url.PathEscape(project) + "/tasks/" + url.PathEscape(id)
}
func (s *Server) globalTasks(ctx context.Context, q string, offset int) ([]UniversalSearchItem, error) {
	out := []UniversalSearchItem{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT id,COALESCE(project_id,''),title,instructions,summary,complete_note FROM tasks WHERE strpos(lower(title||' '||instructions||' '||summary||' '||complete_note),$1)>0 ORDER BY CASE WHEN strpos(lower(title),$1)>0 THEN 0 ELSE 1 END,updated_at DESC,id LIMIT 6 OFFSET $2`, q, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, project, title, instructions, summary, note string
			if err = rows.Scan(&id, &project, &title, &instructions, &summary, &note); err != nil {
				return nil, err
			}
			out = append(out, UniversalSearchItem{ID: id, Title: title, Snippet: memorySnippet(instructions+" "+summary+" "+note, q), URL: taskSearchURL(project, id)})
		}
		return out, rows.Err()
	}
	if s.taskSvc == nil {
		return out, nil
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := strings.Contains(strings.ToLower(tasks[i].Title), q), strings.Contains(strings.ToLower(tasks[j].Title), q)
		if a != b {
			return a
		}
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})
	for _, t := range tasks {
		body := t.Instructions + " " + t.Summary + " " + t.CompleteNote
		if strings.Contains(strings.ToLower(t.Title+" "+body), q) {
			out = append(out, UniversalSearchItem{ID: t.ID, Title: t.Title, Snippet: memorySnippet(body, q), URL: taskSearchURL(t.ProjectID, t.ID)})
		}
	}
	return searchPage(out, offset), nil
}
func (s *Server) globalConversations(ctx context.Context, q string, offset int) ([]UniversalSearchItem, error) {
	out := []UniversalSearchItem{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT r.id,COALESCE(NULLIF(c.name,''),NULLIF(c.email,''),r.title),COALESCE(m.content,''),COALESCE(m.id::text,'') FROM project_rooms r
  LEFT JOIN crm_contacts c ON c.session_uuid=r.id
  LEFT JOIN LATERAL (SELECT id,content FROM project_messages WHERE room_id=r.id AND strpos(lower(content),$1)>0 ORDER BY created_at DESC,id DESC LIMIT 1) m ON true
  WHERE (r.topic LIKE 'agent:%' OR r.topic LIKE 'agent-topic:%' OR c.id IS NOT NULL)
  AND (strpos(lower(r.title||' '||COALESCE(c.name,'')||' '||COALESCE(c.email,'')),$1)>0 OR m.content IS NOT NULL)
  ORDER BY CASE WHEN strpos(lower(r.title||' '||COALESCE(c.name,'')),$1)>0 THEN 0 ELSE 1 END,r.updated_at DESC,r.id LIMIT 6 OFFSET $2`, q, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, body, messageID string
			if err = rows.Scan(&id, &title, &body, &messageID); err != nil {
				return nil, err
			}
			text, _ := decodeAttachmentMessage(body)
			target := "/inbox/" + url.PathEscape(id)
			if messageID != "" {
				target += "?message=" + url.QueryEscape(messageID)
			}
			out = append(out, UniversalSearchItem{ID: id, Title: title, Snippet: memorySnippet(text, q), URL: target})
		}
		return out, rows.Err()
	}
	if s.projectStudioRepo == nil {
		return out, nil
	}
	rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
	if err != nil {
		return nil, err
	}
	contacts, err := s.crmList(ctx)
	if err != nil {
		return nil, err
	}
	customer := map[string]CRMContact{}
	for _, c := range contacts {
		customer[c.SessionUUID] = c
	}
	sort.SliceStable(rooms, func(i, j int) bool { return rooms[i].UpdatedAt.After(rooms[j].UpdatedAt) })
	for _, room := range rooms {
		c, isCustomer := customer[room.ID]
		if roomAgentID(room.Topic) == "" && !isCustomer {
			continue
		}
		title := room.Title
		if isCustomer {
			title = contactSearchTitle(c.Name, c.Email)
		}
		body, messageID := "", ""
		matched := strings.Contains(strings.ToLower(room.Title+" "+c.Name+" "+c.Email), q)
		messages, e := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
		if e != nil {
			return nil, e
		}
		for i := len(messages) - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(messages[i].Content), q) {
				body, _ = decodeAttachmentMessage(messages[i].Content)
				messageID = messages[i].ID
				matched = true
				break
			}
		}
		if matched {
			target := "/inbox/" + url.PathEscape(room.ID)
			if messageID != "" {
				target += "?message=" + url.QueryEscape(messageID)
			}
			out = append(out, UniversalSearchItem{ID: room.ID, Title: title, Snippet: memorySnippet(body, q), URL: target})
		}
	}
	return searchPage(out, offset), nil
}
func (s *Server) handleUniversalSearch(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if r.URL.Query().Get("offset") == "" {
		offset = 0
		err = nil
	}
	if err != nil || offset < 0 || offset > 1000 || utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 200 {
		respondJSONError(w, 400, "Use a search of 2–200 characters")
		return
	}
	q = strings.ToLower(q)
	// A bounded deadline and lightweight queries keep typing from spawning heavy
	// task-detail loads or agent calls. Every request uses the owner boundary.
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	groups := []UniversalSearchGroup{}
	getters := []struct {
		kind string
		get  func(context.Context, string, int) ([]UniversalSearchItem, error)
	}{
		{"contacts", func(ctx context.Context, q string, offset int) ([]UniversalSearchItem, error) {
			contacts, e := s.crmSearch(ctx, q, "", "", offset)
			if e != nil {
				return nil, e
			}
			items := []UniversalSearchItem{}
			for _, c := range contacts {
				snippet := c.Email
				if c.SearchNote != nil {
					snippet = c.SearchNote.Snippet
				}
				target := "/contacts?contact=" + url.QueryEscape(c.ID)
				if c.SearchNote != nil {
					target += "&note=" + url.QueryEscape(c.SearchNote.ID) + "&note_query=" + url.QueryEscape(q)
				}
				items = append(items, UniversalSearchItem{ID: c.ID, Title: contactSearchTitle(c.Name, c.Email), Snippet: snippet, URL: target})
			}
			if len(items) > 6 {
				items = items[:6]
			}
			return items, nil
		}},
		{"notes", s.globalNotes}, {"conversations", s.globalConversations}, {"projects", s.globalProjects}, {"tasks", s.globalTasks},
	}
	for _, getter := range getters {
		items, e := getter.get(ctx, q, offset)
		group := UniversalSearchGroup{Kind: getter.kind, Items: []UniversalSearchItem{}}
		if e != nil {
			group.Error = "Search is temporarily unavailable for this category."
		} else {
			group.HasMore = len(items) > 5
			if group.HasMore {
				items = items[:5]
			}
			group.Items = items
		}
		groups = append(groups, group)
	}
	if s.extensions != nil {
		for _, result := range s.extensions.Search(ctx, sdk.SearchRequest{Query: q, Offset: offset, Limit: 6, Scope: sdk.Scope{OwnerID: owner}}) {
			group := UniversalSearchGroup{Kind: "extension:" + result.ID, Label: result.Label, Items: []UniversalSearchItem{}}
			if result.Error != nil {
				group.Error = "Search is temporarily unavailable for this category."
			} else {
				for _, item := range result.Items {
					group.Items = append(group.Items, UniversalSearchItem{ID: item.ID, Title: item.Title, Snippet: item.Snippet, URL: item.URL})
				}
				group.HasMore = len(group.Items) > 5
				if group.HasMore {
					group.Items = group.Items[:5]
				}
			}
			groups = append(groups, group)
		}
	}
	respondJSON(w, 200, map[string]any{"groups": groups})
}
