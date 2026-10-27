package sponsorship

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatIDR(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "Rp 0"},
		{500, "Rp 500"},
		{10000, "Rp 10.000"},
		{25000, "Rp 25.000"},
		{100000, "Rp 100.000"},
		{2500000, "Rp 2.500.000"},
		{10000000, "Rp 10.000.000"},
		{-50000, "Rp -50.000"},
	}

	for _, tc := range tests {
		actual := FormatIDR(tc.input)
		if actual != tc.expected {
			t.Errorf("FormatIDR(%d) = %s; want %s", tc.input, actual, tc.expected)
		}
	}
}

func TestRankAndTierBadges(t *testing.T) {
	if r1 := GetRankDisplay(1); !strings.Contains(r1, "🥇") || !strings.Contains(r1, "#1") {
		t.Errorf("unexpected rank 1 display: %s", r1)
	}
	if r2 := GetRankDisplay(2); !strings.Contains(r2, "🥈") || !strings.Contains(r2, "#2") {
		t.Errorf("unexpected rank 2 display: %s", r2)
	}
	if r3 := GetRankDisplay(3); !strings.Contains(r3, "🥉") || !strings.Contains(r3, "#3") {
		t.Errorf("unexpected rank 3 display: %s", r3)
	}
	if r4 := GetRankDisplay(4); !strings.Contains(r4, "⭐") || !strings.Contains(r4, "#4") {
		t.Errorf("unexpected rank 4 display: %s", r4)
	}
	if r10 := GetRankDisplay(10); r10 != "#10" {
		t.Errorf("unexpected rank 10 display: %s", r10)
	}

	// Tiers
	t1 := GetSponsorTierBadge(1, 5000000)
	if !strings.Contains(t1, "Platinum") {
		t.Errorf("expected Platinum for rank 1, got %s", t1)
	}

	t2 := GetSponsorTierBadge(2, 2500000)
	if !strings.Contains(t2, "Gold") {
		t.Errorf("expected Gold for rank 2, got %s", t2)
	}

	t6High := GetSponsorTierBadge(6, 1500000)
	if !strings.Contains(t6High, "Gold Backer") {
		t.Errorf("expected Gold Backer for position 6 with 1.5M, got %s", t6High)
	}

	t6Standard := GetSponsorTierBadge(6, 25000)
	if !strings.Contains(t6Standard, "Founding Backer") {
		t.Errorf("expected Founding Backer for standard contribution, got %s", t6Standard)
	}
}

func TestFormatSafeMarkdownLink(t *testing.T) {
	// 1. Normal HTTPS URL
	l1 := FormatSafeMarkdownLink("Acme Corp", "https://acme.com")
	if l1 != "[**Acme Corp**](https://acme.com)" {
		t.Errorf("unexpected link: %s", l1)
	}

	// 2. HTTP without protocol (auto prepends https)
	l2 := FormatSafeMarkdownLink("Dropsugar", "dropsugar.com/cuan")
	if l2 != "[**Dropsugar**](https://dropsugar.com/cuan)" {
		t.Errorf("unexpected link without protocol: %s", l2)
	}

	// 3. Dangerous javascript: scheme -> falls back to plain bold text
	l3 := FormatSafeMarkdownLink("Hacker Site", "javascript:alert('xss')")
	if l3 != "**Hacker Site**" {
		t.Errorf("expected safe fallback for javascript: scheme, got: %s", l3)
	}

	// 4. Pipes in brand name
	l4 := FormatSafeMarkdownLink("Company | Tech", "https://company.tech")
	if !strings.Contains(l4, "Company - Tech") {
		t.Errorf("expected pipe to be replaced with dash, got: %s", l4)
	}

	// 5. Empty URL
	l5 := FormatSafeMarkdownLink("Anonymous Backer", "")
	if l5 != "**Anonymous Backer**" {
		t.Errorf("expected bold text for empty URL, got: %s", l5)
	}
}

func TestRenderSponsorsMarkdown(t *testing.T) {
	// Empty sponsors
	emptyOut := RenderSponsorsMarkdown(nil, "v1.0 Founding Backers")
	if !strings.Contains(emptyOut, "No sponsors are registered") {
		t.Errorf("expected empty state message in empty render, got: %s", emptyOut)
	}

	// Multiple sponsors
	now := time.Now().UTC()
	sponsors := []*Sponsor{
		{
			ID:         "sp_1",
			BrandName:  "TechNova Inc",
			WebsiteURL: "https://technova.io",
			Slogan:     "Cloud Automation Solutions",
			AmountIDR:  5000000,
			PaidAt:     &now,
		},
		{
			ID:         "sp_2",
			BrandName:  "DevHub Studio",
			WebsiteURL: "https://devhub.dev",
			Slogan:     "Developer tooling for teams",
			AmountIDR:  2500000,
			PaidAt:     &now,
		},
	}

	out := RenderSponsorsMarkdown(sponsors, "v1.0 Founding Backers")
	if !strings.Contains(out, "TechNova Inc") || !strings.Contains(out, "DevHub Studio") {
		t.Errorf("expected sponsor names in markdown, got: %s", out)
	}
	if !strings.Contains(out, "Rp 5.000.000") || !strings.Contains(out, "Rp 2.500.000") {
		t.Errorf("expected formatted amounts, got: %s", out)
	}
	if !strings.Contains(out, "| 🥇 #1 |") || !strings.Contains(out, "| 🥈 #2 |") {
		t.Errorf("expected leaderboard ranks, got: %s", out)
	}
}

func TestUpdateReadmeContent_Idempotency(t *testing.T) {
	sampleInitialReadme := `# Cuan

> Agentic project management for self-hosted teams.

## Architecture

Monorepo in Go and React.

## License

Apache-2.0
`

	renderedMarkdown := `### v1.0 Founding Backers

| Peringkat | Brand & Tautan | Slogan | Kontribusi | Tier |
| :---: | :--- | :--- | :--- | :--- |
| 🥇 #1 | [**Acme Corp**](https://acme.com) | Scale with ease | ` + "`Rp 5.000.000`" + ` | 🌟 Platinum Founding Backer |
`

	// 1. First run: should insert "## Backers & Sponsors" and markers before "## License"
	updated1, changed1, err := UpdateReadmeContent(sampleInitialReadme, renderedMarkdown)
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	if !changed1 {
		t.Fatalf("expected changed1 to be true")
	}
	if !strings.Contains(updated1, MarkerSponsorsStart) || !strings.Contains(updated1, MarkerSponsorsEnd) {
		t.Fatalf("expected markers in updated README")
	}
	if !strings.Contains(updated1, "Acme Corp") {
		t.Fatalf("expected Acme Corp in updated README")
	}
	if !strings.Contains(updated1, "## Architecture") || !strings.Contains(updated1, "## License") {
		t.Fatalf("original sections corrupted")
	}

	// 2. Second run with EXACT SAME content: should be strictly idempotent (changed == false, content identical)
	updated2, changed2, err := UpdateReadmeContent(updated1, renderedMarkdown)
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}
	if changed2 {
		t.Fatalf("expected changed2 to be false (idempotent)")
	}
	if updated2 != updated1 {
		t.Fatalf("expected updated2 to match updated1 exactly")
	}

	// 3. Third run: simulated 5 consecutive identical calls
	current := updated2
	for i := 0; i < 5; i++ {
		next, changed, err := UpdateReadmeContent(current, renderedMarkdown)
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
		if changed {
			t.Fatalf("iteration %d should not have reported change", i)
		}
		if next != current {
			t.Fatalf("iteration %d changed content unexpectedly", i)
		}
		current = next
	}

	// 4. Update with a second sponsor added
	newRendered := `### v1.0 Founding Backers

| Peringkat | Brand & Tautan | Slogan | Kontribusi | Tier |
| :---: | :--- | :--- | :--- | :--- |
| 🥇 #1 | [**Acme Corp**](https://acme.com) | Scale with ease | ` + "`Rp 5.000.000`" + ` | 🌟 Platinum Founding Backer |
| 🥈 #2 | [**Beta Labs**](https://betalabs.com) | AI for developers | ` + "`Rp 2.500.000`" + ` | 🥇 Gold Founding Backer |
`
	updatedWithBeta, changedWithBeta, err := UpdateReadmeContent(current, newRendered)
	if err != nil {
		t.Fatalf("update with Beta failed: %v", err)
	}
	if !changedWithBeta {
		t.Fatalf("expected change with Beta Labs added")
	}
	if !strings.Contains(updatedWithBeta, "Beta Labs") {
		t.Fatalf("expected Beta Labs in content")
	}
	// Verify again markers are preserved and no duplicates
	if strings.Count(updatedWithBeta, MarkerSponsorsStart) != 1 {
		t.Fatalf("expected exactly 1 start marker, found %d", strings.Count(updatedWithBeta, MarkerSponsorsStart))
	}
	if strings.Count(updatedWithBeta, MarkerSponsorsEnd) != 1 {
		t.Fatalf("expected exactly 1 end marker, found %d", strings.Count(updatedWithBeta, MarkerSponsorsEnd))
	}
	if strings.Count(updatedWithBeta, "## Backers & Sponsors") != 1 {
		t.Fatalf("expected exactly 1 heading, found %d", strings.Count(updatedWithBeta, "## Backers & Sponsors"))
	}
}

func TestSyncer_EndToEndSync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-syncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	readmePath := filepath.Join(tempDir, "README.md")
	initialReadme := `# Cuan Test Repo

## Architecture
Test architecture.

## License
Apache-2.0
`
	if err := os.WriteFile(readmePath, []byte(initialReadme), 0644); err != nil {
		t.Fatalf("failed to write initial readme: %v", err)
	}

	store := NewMemoryStore()
	ctx := context.Background()

	// Initial batch
	batch, err := store.GetBatchBySlug(ctx, "v1.0")
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}

	syncer := NewSyncer(store, SyncerConfig{
		RepoDir:            tempDir,
		ReadmePath:         readmePath,
		SponsorsArchiveDir: filepath.Join(tempDir, "sponsors"),
		BatchSlug:          "v1.0",
	})

	// 1. Initial Sync with zero sponsors (empty state)
	res1, err := syncer.Sync(ctx, "v1.0")
	if err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}
	if !res1.Success {
		t.Fatalf("expected success")
	}
	if res1.SponsorsCount != 0 {
		t.Fatalf("expected 0 sponsors, got %d", res1.SponsorsCount)
	}

	// Verify README was created/updated
	readmeBytes, _ := os.ReadFile(readmePath)
	readmeStr := string(readmeBytes)
	if !strings.Contains(readmeStr, MarkerSponsorsStart) {
		t.Fatalf("missing start marker in readme")
	}
	if !strings.Contains(readmeStr, "No sponsors are registered") {
		t.Fatalf("expected empty state message in readme")
	}

	// Verify sponsors/README.md and sponsors/v1.0.md exist
	archiveReadmePath := filepath.Join(tempDir, "sponsors", "README.md")
	if _, err := os.Stat(archiveReadmePath); err != nil {
		t.Fatalf("sponsors/README.md missing: %v", err)
	}
	batchArchivePath := filepath.Join(tempDir, "sponsors", "v1.0.md")
	if _, err := os.Stat(batchArchivePath); err != nil {
		t.Fatalf("sponsors/v1.0.md missing: %v", err)
	}

	// 2. Add approved sponsors with different amounts
	now := time.Now().UTC()
	sp1 := &Sponsor{
		ID:           "sp_alpha",
		BatchID:      batch.ID,
		BrandName:    "Alpha Cloud",
		WebsiteURL:   "https://alphacloud.com",
		Slogan:       "Fast cloud deployments",
		ContactEmail: "ceo@alphacloud.com",
		Status:       SponsorStatusApproved,
		AmountIDR:    2500000,
		PaidAt:       &now,
		CreatedAt:    now.Add(-1 * time.Hour),
		UpdatedAt:    now,
	}
	_ = store.CreateSponsor(ctx, sp1)

	earlier := now.Add(-2 * time.Hour)
	sp2 := &Sponsor{
		ID:           "sp_mega",
		BatchID:      batch.ID,
		BrandName:    "Mega Scale Corp",
		WebsiteURL:   "https://megascale.io",
		Slogan:       "Enterprise Kubernetes Platform",
		ContactEmail: "sponsor@megascale.io",
		Status:       SponsorStatusApproved,
		AmountIDR:    5000000, // Higher amount -> should be Rank 1!
		PaidAt:       &earlier,
		CreatedAt:    earlier,
		UpdatedAt:    earlier,
	}
	_ = store.CreateSponsor(ctx, sp2)

	// Pending sponsor (should NOT be included)
	spPending := &Sponsor{
		ID:           "sp_pending",
		BatchID:      batch.ID,
		BrandName:    "Pending Co",
		WebsiteURL:   "https://pending.com",
		ContactEmail: "pending@pending.com",
		Status:       SponsorStatusPendingReview,
		AmountIDR:    10000000,
		PaidAt:       &now,
	}
	_ = store.CreateSponsor(ctx, spPending)

	// 3. Sync with approved sponsors
	res2, err := syncer.Sync(ctx, "v1.0")
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if res2.SponsorsCount != 2 {
		t.Fatalf("expected 2 approved sponsors, got %d", res2.SponsorsCount)
	}

	readmeBytes, _ = os.ReadFile(readmePath)
	readmeStr = string(readmeBytes)

	// Mega Scale (5M) must appear before Alpha Cloud (2.5M)
	idxMega := strings.Index(readmeStr, "Mega Scale Corp")
	idxAlpha := strings.Index(readmeStr, "Alpha Cloud")
	if idxMega == -1 || idxAlpha == -1 {
		t.Fatalf("expected both sponsors in README")
	}
	if idxMega > idxAlpha {
		t.Fatalf("expected Mega Scale Corp (Rp 5.000.000) to appear before Alpha Cloud (Rp 2.500.000)")
	}

	// Verify pending sponsor is 100% excluded
	if strings.Contains(readmeStr, "Pending Co") {
		t.Fatalf("security violation: pending sponsor leaked to README.md")
	}

	// 4. Check archive directory files
	batchArchiveBytes, _ := os.ReadFile(batchArchivePath)
	batchArchiveStr := string(batchArchiveBytes)
	if !strings.Contains(batchArchiveStr, "Mega Scale Corp") || !strings.Contains(batchArchiveStr, "Alpha Cloud") {
		t.Fatalf("batch archive missing sponsors")
	}
	if !strings.Contains(batchArchiveStr, "Rp 7.500.000") {
		t.Errorf("expected total contribution sum Rp 7.500.000 in batch archive, got:\n%s", batchArchiveStr)
	}

	// 5. Re-sync without changes should report no file modifications
	res3, err := syncer.Sync(ctx, "v1.0")
	if err != nil {
		t.Fatalf("idempotent sync failed: %v", err)
	}
	if res3.ReadmeUpdated || res3.ArchiveUpdated {
		t.Errorf("expected no updates when data has not changed, got readme:%v archive:%v", res3.ReadmeUpdated, res3.ArchiveUpdated)
	}
}

func TestSyncer_GitCommit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-syncer-git-*")
	if err != nil {
		t.Fatalf("failed to create temp git dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Init git repo
	cmd := exec.Command("git", "init", tempDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init not supported or failed: %s", string(out))
		return
	}

	readmePath := filepath.Join(tempDir, "README.md")
	_ = os.WriteFile(readmePath, []byte("# Repo\n\n## License\nMIT\n"), 0644)

	// Initial commit
	_ = exec.Command("git", "-C", tempDir, "config", "user.name", "Buchin").Run()
	_ = exec.Command("git", "-C", tempDir, "config", "user.email", "buchin@dropsugar.com").Run()
	_ = exec.Command("git", "-C", tempDir, "add", "README.md").Run()
	_ = exec.Command("git", "-C", tempDir, "commit", "-m", "initial commit").Run()

	store := NewMemoryStore()
	ctx := context.Background()
	batch, _ := store.GetBatchBySlug(ctx, "v1.0")

	now := time.Now().UTC()
	sp := &Sponsor{
		ID:           "sp_commit",
		BatchID:      batch.ID,
		BrandName:    "GitCommit Sponsor",
		WebsiteURL:   "https://commitsponsor.org",
		Slogan:       "Open Source Supporter",
		AmountIDR:    1000000,
		Status:       SponsorStatusApproved,
		PaidAt:       &now,
	}
	_ = store.CreateSponsor(ctx, sp)

	syncer := NewSyncer(store, SyncerConfig{
		RepoDir:            tempDir,
		ReadmePath:         readmePath,
		SponsorsArchiveDir: filepath.Join(tempDir, "sponsors"),
		BatchSlug:          "v1.0",
		AutoGitCommit:      true,
		AuthorName:         "Buchin",
		AuthorEmail:        "buchin@dropsugar.com",
	})

	res, err := syncer.Sync(ctx, "v1.0")
	if err != nil {
		t.Fatalf("sync with git commit failed: %v", err)
	}
	if !res.GitCommitted {
		t.Fatalf("expected GitCommitted to be true")
	}
	if res.CommitHash == "" {
		t.Fatalf("expected non-empty commit hash")
	}

	// Verify git author
	logCmd := exec.Command("git", "-C", tempDir, "log", "-n", "1", "--format=%an <%ae>")
	out, err := logCmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	authorLine := strings.TrimSpace(string(out))
	if authorLine != "Buchin <buchin@dropsugar.com>" {
		t.Fatalf("expected author Buchin <buchin@dropsugar.com>, got: %s", authorLine)
	}
}
