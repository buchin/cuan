package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

type inboxTeamAction struct {
	Action string `json:"action"` // "list", "invite", "remove"
	Email  string `json:"email,omitempty"`
	Role   string `json:"role,omitempty"` // "owner", "member"
	ID     string `json:"id,omitempty"`
}

func parseInboxTeam(content string) (string, *inboxTeamAction, error) {
	const marker = "```cuan-team\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid team action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxTeamAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	if a.Action == "" {
		a.Action = "list"
	}
	if a.Action == "invite" && strings.TrimSpace(a.Email) == "" {
		return "", nil, errors.New("Email is required to invite a team member.")
	}
	if a.Action == "remove" && strings.TrimSpace(a.ID) == "" {
		return "", nil, errors.New("ID is required to remove a team member or invite.")
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxTeam(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxTeam(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.authSvc == nil {
		return cleanText + "\n\n⚠️ Layanan tim tidak tersedia (auth service unconfigured)."
	}

	switch action.Action {
	case "list":
		members, invites, err := s.authSvc.ListTeamMembers(ctx)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memuat daftar tim: %v", err)
		}
		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString("### Anggota Tim Terdaftar\n")
		if len(members) == 0 {
			sb.WriteString("_Belum ada anggota tim tambahan terdaftar._\n")
		} else {
			for _, m := range members {
				roleLabel := "Member"
				if strings.EqualFold(m.Role, auth.RoleOwner) {
					roleLabel = "Owner"
				}
				name := m.Name
				if name == "" {
					name = m.Email
				}
				sb.WriteString(fmt.Sprintf("• **%s** (`%s`) — Peran: %s\n", name, m.Email, roleLabel))
			}
		}

		if len(invites) > 0 {
			sb.WriteString("\n### Undangan Menunggu Konfirmasi\n")
			for _, inv := range invites {
				sb.WriteString(fmt.Sprintf("• `%s` (Peran: %s) — Berlaku hingga %s\n",
					inv.Email, inv.Role, inv.ExpiresAt.Format("02 Jan 2006 15:04")))
			}
		}
		return strings.TrimSpace(sb.String())

	case "invite":
		ownerID := q.OwnerID
		if ownerID == "" {
			ownerID = "owner_admin"
		}
		role := strings.ToLower(strings.TrimSpace(action.Role))
		if role == "" {
			role = auth.RoleMember
		}
		resp, err := s.authSvc.CreateInvite(ctx, ownerID, action.Email, role)
		if err != nil {
			if errors.Is(err, auth.ErrUserAlreadyExists) {
				return cleanText + fmt.Sprintf("\n\n⚠️ Pengguna dengan email `%s` sudah terdaftar sebagai anggota tim.", action.Email)
			}
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal membuat undangan tim: %v", err)
		}
		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Undangan untuk **%s** berhasil dibuat sebagai **%s**.\n\n", resp.Invitation.Email, resp.Invitation.Role))
		if resp.Token != "" {
			sb.WriteString(fmt.Sprintf("• **Token Undangan:** `%s`\n", resp.Token))
		}
		if resp.InviteURL != "" {
			sb.WriteString(fmt.Sprintf("• **Tautan Undangan:** %s\n", resp.InviteURL))
		}
		sb.WriteString(fmt.Sprintf("• **Kadaluarsa:** %s\n\n", resp.Invitation.ExpiresAt.Format("02 Jan 2006 15:04 UTC")))
		sb.WriteString("Tautan undangan ini dapat langsung dikirimkan ke calon anggota tim untuk aktivasi akun.")
		return strings.TrimSpace(sb.String())

	case "remove":
		targetID := strings.TrimSpace(action.ID)
		ownerID := q.OwnerID
		if ownerID != "" && ownerID == targetID {
			return cleanText + "\n\n⚠️ Tidak dapat menghapus akun Owner yang sedang aktif."
		}
		members, invites, listErr := s.authSvc.ListTeamMembers(ctx)
		if listErr == nil {
			if ownerID != "" && ownerID != "owner_admin" {
				for _, m := range members {
					if m.ID == ownerID && strings.EqualFold(strings.TrimSpace(m.Role), auth.RoleMember) {
						return cleanText + "\n\n⚠️ Hanya pemilik workspace (Owner) yang dapat menghapus anggota tim atau undangan."
					}
				}
			}
			for _, inv := range invites {
				if inv.ID == targetID {
					if err := s.authSvc.DeleteInvite(ctx, targetID); err != nil {
						return cleanText + fmt.Sprintf("\n\n⚠️ Gagal membatalkan undangan: %v", err)
					}
					return cleanText + fmt.Sprintf("\n\n✓ Undangan untuk `%s` berhasil dibatalkan.", inv.Email)
				}
			}
		}
		if err := s.authSvc.DeleteMember(ctx, targetID); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menghapus anggota tim: %v", err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ Anggota tim (`%s`) berhasil dihapus.", targetID)

	default:
		return cleanText + fmt.Sprintf("\n\n⚠️ Aksi tim '%s' tidak didukung.", action.Action)
	}
}
