package sponsorship

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MarkerSponsorsStart = "<!-- SPONSORS:START -->"
	MarkerSponsorsEnd   = "<!-- SPONSORS:END -->"
	DefaultBatchSlug    = "v1.0"
)

var (
	ErrEmptyReadmeContent = errors.New("README content cannot be empty")
)

// SyncerConfig contains configuration for the README and archive syncer runner.
type SyncerConfig struct {
	RepoDir            string
	ReadmePath         string
	SponsorsArchiveDir string
	BatchSlug          string
	AutoGitCommit      bool
	AuthorName         string
	AuthorEmail        string
}

// SyncResult captures the output and status of a synchronization cycle.
type SyncResult struct {
	Success        bool      `json:"success"`
	BatchSlug      string    `json:"batch_slug"`
	SponsorsCount  int       `json:"sponsors_count"`
	ReadmeUpdated  bool      `json:"readme_updated"`
	ArchiveUpdated bool      `json:"archive_updated"`
	GitCommitted   bool      `json:"git_committed"`
	CommitHash     string    `json:"commit_hash,omitempty"`
	FilesModified  []string  `json:"files_modified"`
	Message        string    `json:"message"`
	SyncedAt       time.Time `json:"synced_at"`
}

// Syncer orchestrates synchronization between approved sponsors and markdown documents.
type Syncer struct {
	store Store
	cfg   SyncerConfig
	mu    sync.Mutex
}

// NewSyncer constructs a new Syncer instance with safe defaults.
func NewSyncer(store Store, cfg SyncerConfig) *Syncer {
	if cfg.RepoDir == "" {
		cfg.RepoDir = "."
	}
	if cfg.ReadmePath == "" {
		cfg.ReadmePath = filepath.Join(cfg.RepoDir, "README.md")
	}
	if cfg.SponsorsArchiveDir == "" {
		cfg.SponsorsArchiveDir = filepath.Join(cfg.RepoDir, "sponsors")
	}
	if cfg.BatchSlug == "" {
		cfg.BatchSlug = DefaultBatchSlug
	}
	if cfg.AuthorName == "" {
		cfg.AuthorName = "Buchin"
	}
	if cfg.AuthorEmail == "" {
		cfg.AuthorEmail = "buchin@dropsugar.com"
	}

	return &Syncer{
		store: store,
		cfg:   cfg,
	}
}

// FormatIDR formats an integer amount to Indonesian Rupiah standard format (e.g. Rp 2.500.000).
func FormatIDR(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	n := len(s)
	if n <= 3 {
		return fmt.Sprintf("Rp %s%s", sign, s)
	}
	var parts []string
	rem := n % 3
	if rem > 0 {
		parts = append(parts, s[:rem])
	}
	for i := rem; i < n; i += 3 {
		parts = append(parts, s[i:i+3])
	}
	return fmt.Sprintf("Rp %s%s", sign, strings.Join(parts, "."))
}

// GetRankDisplay returns a visual badge for the leaderboard position.
func GetRankDisplay(rank int) string {
	switch rank {
	case 1:
		return "🥇 #1"
	case 2:
		return "🥈 #2"
	case 3:
		return "🥉 #3"
	case 4:
		return "⭐ #4"
	case 5:
		return "⭐ #5"
	default:
		return fmt.Sprintf("#%d", rank)
	}
}

// GetSponsorTierBadge returns the sponsor tier badge based on rank and nominal contribution.
func GetSponsorTierBadge(rank int, amount int64) string {
	switch rank {
	case 1:
		return "🌟 Platinum Founding Backer"
	case 2:
		return "🥇 Gold Founding Backer"
	case 3:
		return "🥈 Silver Founding Backer"
	case 4, 5:
		return "🥉 Bronze Founding Backer"
	default:
		if amount >= 1000000 {
			return "🥇 Gold Backer"
		} else if amount >= 250000 {
			return "🥈 Silver Backer"
		} else if amount >= 50000 {
			return "🥉 Bronze Backer"
		}
		return "🎖️ Founding Backer"
	}
}

// FormatSafeMarkdownLink cleanses brand name and builds a safe markdown hyperlink.
func FormatSafeMarkdownLink(brandName, websiteURL string) string {
	cleanName := strings.ReplaceAll(brandName, "|", "-")
	cleanName = strings.ReplaceAll(cleanName, "\n", " ")
	cleanName = strings.TrimSpace(cleanName)
	if cleanName == "" {
		cleanName = "Backer"
	}

	rawURL := strings.TrimSpace(websiteURL)
	if rawURL == "" {
		return fmt.Sprintf("**%s**", cleanName)
	}

	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
		return fmt.Sprintf("**%s**", cleanName)
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	safeURL := strings.ReplaceAll(rawURL, ")", "%29")
	safeURL = strings.ReplaceAll(safeURL, "(", "%28")

	return fmt.Sprintf("[**%s**](%s)", cleanName, safeURL)
}

// CleanTableText strips table-breaking characters such as pipes and newlines.
func CleanTableText(text string) string {
	text = strings.ReplaceAll(text, "|", "-")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return "-"
	}
	return text
}

// RenderSponsorsMarkdown renders the sponsors section to be placed between markers in README.md.
func RenderSponsorsMarkdown(sponsors []*Sponsor, batchName string) string {
	var sb strings.Builder

	if batchName == "" {
		batchName = "v1.0 Founding Backers"
	}

	if len(sponsors) == 0 {
		sb.WriteString(fmt.Sprintf("### %s\n\n", batchName))
		sb.WriteString("_No sponsors are registered for this batch yet. Become the first founding backer at [cuan.dev/sponsor](https://cuan.dropsugar.com/sponsor)!_\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("### %s\n\n", batchName))
	sb.WriteString("Thank you to the founding backers supporting the independence and sustainability of the Cuan open-source project:\n\n")
	sb.WriteString("| Rank | Brand & link | Slogan | Contribution | Tier |\n")
	sb.WriteString("| :---: | :--- | :--- | :--- | :--- |\n")

	for i, sp := range sponsors {
		rank := i + 1
		rankDisplay := GetRankDisplay(rank)
		brandLink := FormatSafeMarkdownLink(sp.BrandName, sp.WebsiteURL)
		slogan := CleanTableText(sp.Slogan)
		amount := FormatIDR(sp.AmountIDR)
		tier := GetSponsorTierBadge(rank, sp.AmountIDR)

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | `%s` | %s |\n", rankDisplay, brandLink, slogan, amount, tier))
	}

	sb.WriteString("\n> Want your brand listed in the GitHub README and public directory `/sponsors`? Support Cuan development through [Founding Sponsorship](https://cuan.dropsugar.com/sponsor) (Pay-What-You-Want from IDR 10,000).\n")

	return sb.String()
}

// UpdateReadmeContent replaces or inserts the sponsors section into existing README content idempotently.
func UpdateReadmeContent(existingContent string, renderedMarkdown string) (string, bool, error) {
	trimmedRendered := strings.TrimSpace(renderedMarkdown)
	replacementBlock := fmt.Sprintf("%s\n\n%s\n\n%s", MarkerSponsorsStart, trimmedRendered, MarkerSponsorsEnd)

	startIdx := strings.Index(existingContent, MarkerSponsorsStart)
	endIdx := strings.Index(existingContent, MarkerSponsorsEnd)

	if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
		// Markers exist -> replace between them (including markers)
		fullEnd := endIdx + len(MarkerSponsorsEnd)
		currentBlock := existingContent[startIdx:fullEnd]
		if currentBlock == replacementBlock {
			return existingContent, false, nil
		}
		updated := existingContent[:startIdx] + replacementBlock + existingContent[fullEnd:]
		return updated, true, nil
	}

	// Markers do not exist, check for heading "## Backers & Sponsors"
	headerIdx := strings.Index(existingContent, "## Backers & Sponsors")
	if headerIdx != -1 {
		lineEnd := strings.Index(existingContent[headerIdx:], "\n")
		insertPoint := headerIdx + len("## Backers & Sponsors")
		if lineEnd != -1 {
			insertPoint = headerIdx + lineEnd
		}
		updated := existingContent[:insertPoint] + "\n\n" + replacementBlock + existingContent[insertPoint:]
		return updated, true, nil
	}

	// Look for "## License"
	licenseIdx := strings.Index(existingContent, "## License")
	if licenseIdx != -1 {
		prefix := existingContent[:licenseIdx]
		newSection := fmt.Sprintf("## Backers & Sponsors\n\n%s\n\n---\n\n", replacementBlock)
		updated := prefix + newSection + existingContent[licenseIdx:]
		return updated, true, nil
	}

	// Fallback: append at the end
	updated := strings.TrimRight(existingContent, "\n") + "\n\n## Backers & Sponsors\n\n" + replacementBlock + "\n"
	return updated, true, nil
}

// RenderArchiveReadme generates the master index document for the /sponsors directory.
func RenderArchiveReadme(batches []*Batch, allSponsors []*Sponsor) string {
	var sb strings.Builder
	sb.WriteString("# Cuan Backers & Sponsors Archive\n\n")
	sb.WriteString("> Welcome to the Cuan archive. Contributors, donors, and sponsors supporting open-source Cuan are recorded in this repository’s version history.\n\n")

	sb.WriteString("## Integrity & moderation\n\n")
	sb.WriteString("Each listed sponsor has been reviewed by the Cuan owner. Cuan has a zero-tolerance policy for online gambling, pornography, fraud, and harmful material. Violating brands are rejected and permanently blocked.\n\n")

	sb.WriteString("## Sponsorship batches\n\n")
	sb.WriteString("| Batch | Program | Status | Archive | Sponsors | Contributions |\n")
	sb.WriteString("| :---: | :--- | :---: | :---: | :---: | :---: |\n")

	batchMap := make(map[string][]*Sponsor)
	for _, sp := range allSponsors {
		batchMap[sp.BatchID] = append(batchMap[sp.BatchID], sp)
	}

	if len(batches) == 0 {
		sb.WriteString("| `v1.0` | v1.0 Founding Backers | 🟢 Active | [v1.0.md](v1.0.md) | 0 | Rp 0 |\n")
	} else {
		for _, b := range batches {
			sps := batchMap[b.ID]
			var sum int64
			for _, s := range sps {
				sum += s.AmountIDR
			}
			count := len(sps)
			docLink := fmt.Sprintf("[%s.md](%s.md)", b.BatchSlug, b.BatchSlug)
			statusBadge := "🟢 Active"
			if b.Status == "closed" {
				statusBadge = "🔒 Closed"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %d | %s |\n", b.BatchSlug, b.Name, statusBadge, docLink, count, FormatIDR(sum)))
		}
	}

	sb.WriteString("\n## Become a founding backer\n\n")
	sb.WriteString("Support Cuan through Pay-What-You-Want sponsorship (minimum IDR 10,000) at [cuan.dev/sponsor](https://cuan.dropsugar.com/sponsor).\n")

	return sb.String()
}

// RenderBatchArchiveMarkdown generates the per-batch permanent ledger file (e.g. sponsors/v1.0.md).
func RenderBatchArchiveMarkdown(batch *Batch, sponsors []*Sponsor, lastSynced ...time.Time) string {
	var sb strings.Builder
	batchTitle := "v1.0 Founding Backers"
	batchSlug := DefaultBatchSlug
	if batch != nil {
		if batch.Name != "" {
			batchTitle = batch.Name
		}
		if batch.BatchSlug != "" {
			batchSlug = batch.BatchSlug
		}
	}

	var totalAmount int64
	for _, sp := range sponsors {
		totalAmount += sp.AmountIDR
	}

	syncedAt := time.Now().UTC()
	if len(lastSynced) > 0 && !lastSynced[0].IsZero() {
		syncedAt = lastSynced[0].UTC()
	}

	sb.WriteString(fmt.Sprintf("# Backers archive: %s\n\n", batchTitle))
	sb.WriteString(fmt.Sprintf("- **Batch Slug:** `%s`\n", batchSlug))
	sb.WriteString(fmt.Sprintf("- **Status:** Verified & active\n"))
	sb.WriteString(fmt.Sprintf("- **Registered sponsors:** %d Brand\n", len(sponsors)))
	sb.WriteString(fmt.Sprintf("- **Total contributions:** %s\n", FormatIDR(totalAmount)))
	sb.WriteString(fmt.Sprintf("- **Last synced:** %s\n\n", syncedAt.Format(time.RFC3339)))

	sb.WriteString("## Founding backers\n\n")
	if len(sponsors) == 0 {
		sb.WriteString("_No approved sponsors in this batch yet._\n")
	} else {
		sb.WriteString("| Rank | Brand | Website | Slogan | Contribution | Verified on | Tier |\n")
		sb.WriteString("| :---: | :--- | :--- | :--- | :---: | :---: | :--- |\n")

		for i, sp := range sponsors {
			rank := i + 1
			rankDisplay := GetRankDisplay(rank)
			brandLink := FormatSafeMarkdownLink(sp.BrandName, sp.WebsiteURL)
			cleanURL := CleanTableText(sp.WebsiteURL)
			slogan := CleanTableText(sp.Slogan)
			amount := FormatIDR(sp.AmountIDR)
			paidAt := "-"
			if sp.PaidAt != nil {
				paidAt = sp.PaidAt.UTC().Format("2006-01-02 15:04 UTC")
			}
			tier := GetSponsorTierBadge(rank, sp.AmountIDR)

			sb.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s | `%s` | %s | %s |\n", rankDisplay, brandLink, cleanURL, slogan, amount, paidAt, tier))
		}
	}

	sb.WriteString("\n---\n*Dokumen arsip ini di-generate secara otomatis oleh Cuan Sponsorship Syncer Runner dan diabadikan dalam Git repository.*\n")

	return sb.String()
}

// Sync executes complete synchronization: README.md and sponsors/ archive directory.
func (s *Syncer) Sync(ctx context.Context, batchSlug string) (*SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if batchSlug == "" {
		batchSlug = s.cfg.BatchSlug
	}
	if batchSlug == "" {
		batchSlug = DefaultBatchSlug
	}

	result := &SyncResult{
		BatchSlug:     batchSlug,
		FilesModified: make([]string, 0),
		SyncedAt:      time.Now().UTC(),
	}

	// 1. Fetch approved sponsors
	sponsors, err := s.store.ListApprovedSponsors(ctx, batchSlug)
	if err != nil {
		return nil, fmt.Errorf("list approved sponsors: %w", err)
	}
	result.SponsorsCount = len(sponsors)

	batches, _ := s.store.ListBatches(ctx)
	var activeBatch *Batch
	for _, b := range batches {
		if b.BatchSlug == batchSlug {
			activeBatch = b
			break
		}
	}
	if activeBatch == nil {
		b, err := s.store.GetBatchBySlug(ctx, batchSlug)
		if err == nil {
			activeBatch = b
		}
	}
	batchName := "v1.0 Founding Backers"
	if activeBatch != nil && activeBatch.Name != "" {
		batchName = activeBatch.Name
	}

	// 2. Synchronize README.md
	readmePath := s.cfg.ReadmePath
	if readmePath == "" {
		readmePath = filepath.Join(s.cfg.RepoDir, "README.md")
	}

	var existingReadmeContent string
	if readmeBytes, err := os.ReadFile(readmePath); err == nil {
		existingReadmeContent = string(readmeBytes)
	}

	renderedSponsors := RenderSponsorsMarkdown(sponsors, batchName)
	updatedReadme, readmeChanged, err := UpdateReadmeContent(existingReadmeContent, renderedSponsors)
	if err != nil {
		return nil, fmt.Errorf("update readme content: %w", err)
	}

	if readmeChanged || existingReadmeContent == "" {
		if err := os.MkdirAll(filepath.Dir(readmePath), 0755); err == nil {
			if err := os.WriteFile(readmePath, []byte(updatedReadme), 0644); err != nil {
				return nil, fmt.Errorf("write readme file: %w", err)
			}
			result.ReadmeUpdated = true
			result.FilesModified = append(result.FilesModified, readmePath)
		}
	}

	// 3. Synchronize sponsors/ archive directory
	archiveDir := s.cfg.SponsorsArchiveDir
	if archiveDir == "" {
		archiveDir = filepath.Join(s.cfg.RepoDir, "sponsors")
	}

	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("create sponsors archive dir: %w", err)
	}

	// 3a. sponsors/README.md master archive
	archiveReadmePath := filepath.Join(archiveDir, "README.md")
	archiveReadmeContent := RenderArchiveReadme(batches, sponsors)
	archiveReadmeChanged := true
	if currentBytes, err := os.ReadFile(archiveReadmePath); err == nil {
		if string(currentBytes) == archiveReadmeContent {
			archiveReadmeChanged = false
		}
	}
	if archiveReadmeChanged {
		if err := os.WriteFile(archiveReadmePath, []byte(archiveReadmeContent), 0644); err != nil {
			return nil, fmt.Errorf("write archive readme: %w", err)
		}
		result.ArchiveUpdated = true
		result.FilesModified = append(result.FilesModified, archiveReadmePath)
	}

	// 3b. sponsors/{batch_slug}.md per-batch ledger
	batchDocPath := filepath.Join(archiveDir, fmt.Sprintf("%s.md", batchSlug))
	syncedTimestamp := time.Now().UTC()
	if existingBytes, err := os.ReadFile(batchDocPath); err == nil {
		existingStr := string(existingBytes)
		const tsPrefix = "- **Last synced:** "
		if idx := strings.Index(existingStr, tsPrefix); idx != -1 {
			lineEnd := strings.Index(existingStr[idx:], "\n")
			if lineEnd != -1 {
				timeStr := strings.TrimSpace(existingStr[idx+len(tsPrefix) : idx+lineEnd])
				if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
					testContent := RenderBatchArchiveMarkdown(activeBatch, sponsors, t)
					if testContent == existingStr {
						syncedTimestamp = t
					}
				}
			}
		}
	}
	batchDocContent := RenderBatchArchiveMarkdown(activeBatch, sponsors, syncedTimestamp)
	batchDocChanged := true
	if currentBytes, err := os.ReadFile(batchDocPath); err == nil {
		if string(currentBytes) == batchDocContent {
			batchDocChanged = false
		}
	}
	if batchDocChanged {
		if err := os.WriteFile(batchDocPath, []byte(batchDocContent), 0644); err != nil {
			return nil, fmt.Errorf("write batch archive file: %w", err)
		}
		result.ArchiveUpdated = true
		result.FilesModified = append(result.FilesModified, batchDocPath)
	}

	// 4. Git commit if configured and changes occurred
	if s.cfg.AutoGitCommit && len(result.FilesModified) > 0 {
		commitHash, committed := s.performGitCommit(ctx, result.FilesModified)
		result.GitCommitted = committed
		result.CommitHash = commitHash
	}

	result.Success = true
	result.Message = fmt.Sprintf("Sync complete: %d sponsors listed, %d files updated", result.SponsorsCount, len(result.FilesModified))
	return result, nil
}

// performGitCommit executes git add and git commit with human author identity.
func (s *Syncer) performGitCommit(ctx context.Context, files []string) (string, bool) {
	gitDir := filepath.Join(s.cfg.RepoDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return "", false
	}

	author := fmt.Sprintf("%s <%s>", s.cfg.AuthorName, s.cfg.AuthorEmail)

	// Stage modified files
	for _, f := range files {
		rel, err := filepath.Rel(s.cfg.RepoDir, f)
		if err != nil {
			rel = f
		}
		cmd := exec.CommandContext(ctx, "git", "-C", s.cfg.RepoDir, "add", rel)
		_ = cmd.Run()
	}

	// Commit
	msg := fmt.Sprintf("chore(sponsors): sync README.md and sponsors archive directory\n\nAutomated idempotent sync for %s", s.cfg.BatchSlug)
	cmd := exec.CommandContext(ctx, "git", "-C", s.cfg.RepoDir,
		"-c", fmt.Sprintf("user.name=%s", s.cfg.AuthorName),
		"-c", fmt.Sprintf("user.email=%s", s.cfg.AuthorEmail),
		"commit", "--author", author, "-m", msg,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Nothing to commit or git error
		return "", false
	}

	// Retrieve commit hash
	revCmd := exec.CommandContext(ctx, "git", "-C", s.cfg.RepoDir, "rev-parse", "--short", "HEAD")
	revOut, err := revCmd.Output()
	if err != nil {
		return strings.TrimSpace(string(out)), true
	}
	return strings.TrimSpace(string(revOut)), true
}
