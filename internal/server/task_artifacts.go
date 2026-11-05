package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type taskAgentWorkspaceKey struct{}
type preparedTaskArtifact struct {
	attachment conversationAttachment
	data       []byte
}

func readTaskArtifacts(root *os.Root, taskID, attemptID string, paths []string, evidenceRoots ...*os.Root) ([]preparedTaskArtifact, error) {
	var evidenceRoot *os.Root
	if len(evidenceRoots) > 0 {
		evidenceRoot = evidenceRoots[0]
	}
	if (root == nil && evidenceRoot == nil) || len(paths) > 8 {
		return nil, errors.New("File delivery requires the bound task workspace and at most eight files")
	}
	var result []preparedTaskArtifact
	seen := map[string]bool{}
	total := 0
	for _, sourcePath := range paths {
		path, selectedRoot := sourcePath, root
		if strings.HasPrefix(path, "session:") {
			path = strings.TrimPrefix(path, "session:")
			selectedRoot = evidenceRoot
		}
		if selectedRoot == nil || !filepath.IsLocal(path) || len(path) > 512 || seen[sourcePath] {
			return nil, errors.New("Use unique paths relative to this task's workspace")
		}
		seen[sourcePath] = true
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			lower := strings.ToLower(part)
			if strings.HasPrefix(part, ".") || strings.HasPrefix(lower, "id_rsa") || strings.HasPrefix(lower, "id_ed25519") || lower == "credentials.json" {
				return nil, errors.New("Hidden files, credentials and private keys cannot be attached")
			}
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".pem", ".key", ".p12", ".pfx":
			return nil, errors.New("Private key files cannot be attached")
		}
		f, err := selectedRoot.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, errors.New("An output file is unavailable in this task's workspace")
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > attachmentLimit {
			f.Close()
			return nil, errors.New("Attach regular output files of at most 10 MB each")
		}
		data, err := io.ReadAll(io.LimitReader(f, attachmentLimit+1))
		f.Close()
		if err != nil || len(data) > attachmentLimit {
			return nil, errors.New("The output file could not be read within the size limit")
		}
		total += len(data)
		if total > 25<<20 {
			return nil, errors.New("Output files exceed the 25 MB total limit")
		}
		hash := sha256.Sum256(data)
		id := delegationMessageID("task-artifact:" + taskID + ":" + attemptID + ":" + sourcePath + ":" + hex.EncodeToString(hash[:]))
		kind := mime.TypeByExtension(filepath.Ext(path))
		if kind == "" {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".md", ".markdown":
				kind = "text/markdown; charset=utf-8"
			case ".txt", ".text", ".log", ".diff", ".patch":
				kind = "text/plain; charset=utf-8"
			case ".json":
				kind = "application/json"
			case ".png":
				kind = "image/png"
			case ".jpg", ".jpeg":
				kind = "image/jpeg"
			case ".webp":
				kind = "image/webp"
			case ".gif":
				kind = "image/gif"
			case ".svg":
				kind = "image/svg+xml"
			case ".ico":
				kind = "image/x-icon"
			case ".bmp":
				kind = "image/bmp"
			case ".html", ".htm":
				kind = "text/html; charset=utf-8"
			case ".csv":
				kind = "text/csv; charset=utf-8"
			case ".tsv":
				kind = "text/tab-separated-values; charset=utf-8"
			default:
				kind = "application/octet-stream"
			}
		}
		a := conversationAttachment{ID: id, Name: filepath.Base(path), Type: kind, Size: int64(len(data)), URL: "/api/v1/chat/attachments/" + id}
		result = append(result, preparedTaskArtifact{a, data})
	}
	return result, nil
}

func (s *Server) taskArtifactOwner(ctx context.Context, d *task.TaskDetailResponse) (string, error) {
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok || d.SourceRoomID == "" || d.SourceMessageID == "" {
		return "", errors.New("This task has no Inbox conversation for file delivery")
	}
	requests, err := store.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return "", errors.New("Conversation ownership is unavailable")
	}
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID && q.OwnerID != "" {
			return q.OwnerID, nil
		}
	}
	if s.taskSvc != nil {
		if launch, err := s.taskSvc.LatestLaunch(ctx, d.ID); err == nil && launch != nil && launch.OwnerID != "" {
			return launch.OwnerID, nil
		}
	}
	return "", errors.New("The task's original conversation owner is unavailable")
}

func (s *Server) publishTaskArtifacts(ctx context.Context, d *task.TaskDetailResponse, owner string, files []preparedTaskArtifact) error {
	return s.publishTaskArtifactsWithKey(ctx, d, owner, files, "task-delivery:"+d.LatestAttempt.ID)
}

func (s *Server) publishTaskArtifactsWithKey(ctx context.Context, d *task.TaskDetailResponse, owner string, files []preparedTaskArtifact, key string) error {
	if len(files) == 0 {
		return nil
	}
	ids := make([]string, 0, len(files))
	for _, file := range files {
		a := file.attachment
		dir, err := attachmentDirectory(owner, a.ID)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		if _, _, err = loadAttachment(owner, a.ID); err != nil {
			if err = os.WriteFile(filepath.Join(dir, "content"+filepath.Ext(a.Name)), file.data, 0600); err != nil {
				return err
			}
			raw, _ := json.Marshal(a)
			if err = os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0600); err != nil {
				return err
			}
		}
		ids = append(ids, a.ID)
	}
	id := delegationMessageID(key)
	content := encodeAttachmentMessage("Files from this work session", ids)
	if existing, err := s.projectStudioRepo.GetMessageByID(ctx, id); err == nil && existing != nil {
		if existing.RoomID == d.SourceRoomID && existing.Content == content {
			return nil
		}
		return errors.New("A different file delivery already exists for this attempt; check it before retrying")
	}
	now := time.Now().UTC()
	author := d.AssignedAgent
	if strings.HasPrefix(key, "task-visual-delivery:") {
		author = db.AgentIDCuan
	}
	m := &db.ProjectMessage{ID: id, RoomID: d.SourceRoomID, AuthorType: "agent", AuthorID: author, AuthorName: "Cuan", Content: content, ReplyToID: &d.SourceMessageID, CreatedAt: now, UpdatedAt: now}
	if err := s.projectStudioRepo.CreateMessage(ctx, m); err != nil {
		return err
	}
	s.publishInbox(d.SourceRoomID)
	return nil
}
