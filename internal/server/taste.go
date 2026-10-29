package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

// TasteCategory denotes the layer of agent taste preference.
type TasteCategory string

const (
	// TasteCategoryBrand represents Brand/Product voice, tone, style, and messaging guidelines.
	TasteCategoryBrand TasteCategory = "brand"
	// TasteCategoryTechnical represents tech stack conventions, runtime constraints, and operational safety.
	TasteCategoryTechnical TasteCategory = "technical"

	// MaxTasteItemsPerCategory limits context memory size per layer to prevent prompt bloat and token overflow.
	MaxTasteItemsPerCategory = 15

	// MaxTasteCandidates sets the safe storage quota limit for suggested/pending candidates.
	MaxTasteCandidates = 30
)

// TasteCandidateStatus represents the review lifecycle state of a suggested taste candidate.
type TasteCandidateStatus string

const (
	// TasteCandidateStatusSuggested indicates a newly extracted preference awaiting user review.
	TasteCandidateStatusSuggested TasteCandidateStatus = "suggested"
	// TasteCandidateStatusPending is an alias for suggested.
	TasteCandidateStatusPending TasteCandidateStatus = "pending"
	// TasteCandidateStatusAccepted indicates user approved candidate and promoted it to active TasteItem.
	TasteCandidateStatusAccepted TasteCandidateStatus = "accepted"
	// TasteCandidateStatusDismissed indicates user rejected/ignored candidate.
	TasteCandidateStatusDismissed TasteCandidateStatus = "dismissed"
)

// ErrTasteItemNotFound is returned when an item cannot be found in the taste store.
var ErrTasteItemNotFound = errors.New("taste item not found")

// ErrTasteCandidateNotFound is returned when a candidate cannot be found in the taste store.
var ErrTasteCandidateNotFound = errors.New("taste candidate not found")

// ErrTasteCandidateAlreadyExists is returned when an identical candidate already exists.
var ErrTasteCandidateAlreadyExists = errors.New("taste candidate already exists")

// TasteCandidate represents an unconfirmed preference candidate extracted from user interactions.
type TasteCandidate struct {
	ID            string    `json:"id"`
	Category      string    `json:"category"` // "brand" or "technical"
	Topic         string    `json:"topic,omitempty"`
	Text          string    `json:"text"`
	Content       string    `json:"content,omitempty"`
	SourceSnippet string    `json:"source_snippet,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	Rationale     string    `json:"rationale,omitempty"`
	Status        string    `json:"status"` // "suggested", "accepted", "dismissed"
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TasteItem represents a discrete taste preference memory point.
type TasteItem struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Topic     string    `json:"topic,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TasteProfile aggregates active taste preferences across Brand and Technical layers.
type TasteProfile struct {
	Items          []TasteItem `json:"items"`
	BrandItems     []TasteItem `json:"brand_items"`
	TechnicalItems []TasteItem `json:"technical_items"`
	Total          int         `json:"total"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type tasteStore struct {
	sync.RWMutex
	profiles   map[string][]TasteItem
	candidates map[string][]TasteCandidate
}

func newTasteStore() *tasteStore {
	return &tasteStore{
		profiles:   make(map[string][]TasteItem),
		candidates: make(map[string][]TasteCandidate),
	}
}

// normalizeCategory normalizes and validates user input for taste categories.
func normalizeCategory(cat string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(cat))
	switch trimmed {
	case "brand", "product", "brand_product", "brand/product", "brand/product taste":
		return string(TasteCategoryBrand), nil
	case "technical", "tech", "technical taste":
		return string(TasteCategoryTechnical), nil
	default:
		return "", fmt.Errorf("Kategori tidak valid: gunakan 'brand' (Brand/Product) atau 'technical' (Technical)")
	}
}

// defaultTasteItems returns clean initial/default preference templates for new profiles or reset.
func defaultTasteItems() []TasteItem {
	now := time.Now().UTC()
	return []TasteItem{
		{
			ID:        "default-brand-tone",
			Category:  string(TasteCategoryBrand),
			Topic:     "Gaya Bahasa & Tone of Voice",
			Content:   "Gunakan bahasa Indonesia yang profesional, ramah, lugas, dan mudah dipahami oleh pengguna non-developer tanpa istilah teknis yang berbelit-belit.",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "default-brand-value",
			Category:  string(TasteCategoryBrand),
			Topic:     "Fokus Nilai Solutif",
			Content:   "Fokus pada solusi bisnis praktis, hasil yang dapat langsung dipakai, dan kejelasan alur kerja bagi kreator dan UMKM.",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "default-brand-action",
			Category:  string(TasteCategoryBrand),
			Topic:     "Komunikasi Terarah & Konkret",
			Content:   "Selalu sertakan langkah konkret atau opsi pilihan yang jelas dan terarah saat memberikan rekomendasi atau proposal.",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "default-tech-stack",
			Category:  string(TasteCategoryTechnical),
			Topic:     "Default Tech Stack",
			Content:   "Default stack: Go untuk backend, React + Vite + TypeScript + Tailwind/shadcn-ui untuk frontend web jika tidak ditentukan lain.",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "default-tech-verify",
			Category:  string(TasteCategoryTechnical),
			Topic:     "Targeted Verification",
			Content:   "Targeted verification wajib: selalu jalankan targeted unit tests dan verifikasi sintaks sebelum menyelesaikan tugas.",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "default-tech-safety",
			Category:  string(TasteCategoryTechnical),
			Topic:     "Git & Deployment Safety",
			Content:   "Commit menggunakan identitas human member, dilarang deploy langsung dari temporary branch, merge ke branch deployment resmi terlebih dahulu.",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// buildTasteProfile constructs a structured TasteProfile from a slice of items.
func buildTasteProfile(items []TasteItem) TasteProfile {
	var brandItems []TasteItem
	var techItems []TasteItem
	latestUpdate := time.Time{}

	for _, it := range items {
		if it.UpdatedAt.After(latestUpdate) {
			latestUpdate = it.UpdatedAt
		}
		if it.Category == string(TasteCategoryBrand) {
			brandItems = append(brandItems, it)
		} else if it.Category == string(TasteCategoryTechnical) {
			techItems = append(techItems, it)
		}
	}

	if brandItems == nil {
		brandItems = []TasteItem{}
	}
	if techItems == nil {
		techItems = []TasteItem{}
	}

	return TasteProfile{
		Items:          items,
		BrandItems:     brandItems,
		TechnicalItems: techItems,
		Total:          len(items),
		UpdatedAt:      latestUpdate,
	}
}

func tastePreferencesPath(owner string) (string, error) {
	p, err := ownerPreferencesPath(owner)
	if err != nil {
		return filepath.Join("data", "preferences", "taste-"+owner+".json"), nil
	}
	return strings.TrimSuffix(p, ".json") + ".taste.json", nil
}

func readTastePreferences(owner string) ([]TasteItem, error) {
	p, err := tastePreferencesPath(owner)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var items []TasteItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func writeTastePreferences(owner string, items []TasteItem) error {
	path, err := tastePreferencesPath(owner)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".taste-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	return err
}

func tasteCandidatesPath(owner string) (string, error) {
	p, err := ownerPreferencesPath(owner)
	if err != nil {
		return filepath.Join("data", "preferences", "taste-candidates-"+owner+".json"), nil
	}
	return strings.TrimSuffix(p, ".json") + ".taste-candidates.json", nil
}

func readTasteCandidates(owner string) ([]TasteCandidate, error) {
	p, err := tasteCandidatesPath(owner)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var items []TasteCandidate
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func writeTasteCandidates(owner string, items []TasteCandidate) error {
	path, err := tasteCandidatesPath(owner)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".taste-candidates-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	return err
}

func normalizeForDeduplication(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r >= 0x80 {
			b.WriteRune(r)
		} else if r == ' ' || r == '\t' || r == '\n' {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (s *Server) getTasteProfile(owner string) (TasteProfile, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	s.taste.RLock()
	cached, ok := s.taste.profiles[ownerKey]
	s.taste.RUnlock()

	if ok && len(cached) > 0 {
		return buildTasteProfile(cached), nil
	}

	s.taste.Lock()
	defer s.taste.Unlock()

	if cached, ok := s.taste.profiles[ownerKey]; ok && len(cached) > 0 {
		return buildTasteProfile(cached), nil
	}

	items, err := readTastePreferences(ownerKey)
	if err == nil && len(items) > 0 {
		s.taste.profiles[ownerKey] = items
		return buildTasteProfile(items), nil
	}

	items = defaultTasteItems()
	s.taste.profiles[ownerKey] = items
	_ = writeTastePreferences(ownerKey, items)

	return buildTasteProfile(items), nil
}

func (s *Server) createTasteItem(owner, category, topic, content string) (TasteItem, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	// Ensure profile is loaded
	_, _ = s.getTasteProfile(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.profiles[ownerKey]

	// Check capacity per category
	catCount := 0
	for _, it := range current {
		if it.Category == category {
			catCount++
		}
	}
	if catCount >= MaxTasteItemsPerCategory {
		return TasteItem{}, fmt.Errorf("Kapasitas maksimum %d item untuk kategori '%s' telah tercapai", MaxTasteItemsPerCategory, category)
	}

	now := time.Now().UTC()
	item := TasteItem{
		ID:        db.GenerateUUID(),
		Category:  category,
		Topic:     topic,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	updated := append(current, item)
	s.taste.profiles[ownerKey] = updated
	_ = writeTastePreferences(ownerKey, updated)

	return item, nil
}

func (s *Server) updateTasteItem(owner, id string, category, topic, content *string) (TasteItem, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	_, _ = s.getTasteProfile(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.profiles[ownerKey]
	foundIdx := -1
	for i, it := range current {
		if it.ID == id {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return TasteItem{}, ErrTasteItemNotFound
	}

	item := current[foundIdx]

	if category != nil && *category != item.Category {
		// Category change: check target category capacity
		targetCount := 0
		for _, it := range current {
			if it.ID != id && it.Category == *category {
				targetCount++
			}
		}
		if targetCount >= MaxTasteItemsPerCategory {
			return TasteItem{}, fmt.Errorf("Kapasitas maksimum %d item untuk kategori '%s' telah tercapai", MaxTasteItemsPerCategory, *category)
		}
		item.Category = *category
	}

	if topic != nil {
		item.Topic = *topic
	}
	if content != nil {
		item.Content = *content
	}

	item.UpdatedAt = time.Now().UTC()
	current[foundIdx] = item
	s.taste.profiles[ownerKey] = current
	_ = writeTastePreferences(ownerKey, current)

	return item, nil
}

func (s *Server) deleteTasteItem(owner, id string) error {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	_, _ = s.getTasteProfile(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.profiles[ownerKey]
	foundIdx := -1
	for i, it := range current {
		if it.ID == id {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return ErrTasteItemNotFound
	}

	updated := append(current[:foundIdx], current[foundIdx+1:]...)
	s.taste.profiles[ownerKey] = updated
	_ = writeTastePreferences(ownerKey, updated)

	return nil
}

func (s *Server) resetTasteProfile(owner string) (TasteProfile, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	s.taste.Lock()
	defer s.taste.Unlock()

	items := defaultTasteItems()
	s.taste.profiles[ownerKey] = items
	_ = writeTastePreferences(ownerKey, items)

	return buildTasteProfile(items), nil
}

// formatBrandTastePrompt formats active Brand/Product Taste preferences for Chat prompt injection.
func (s *Server) formatBrandTastePrompt(ctx context.Context) string {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	profile, err := s.getTasteProfile(ownerID)
	if err != nil || len(profile.BrandItems) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Brand & Product Taste (Preferences & Brand Guidelines):\n")
	b.WriteString("The following user preferences and brand voice guidelines must guide your tone, messaging, and communication:\n")
	for _, item := range profile.BrandItems {
		if strings.TrimSpace(item.Topic) != "" {
			b.WriteString(fmt.Sprintf("- [%s]: %s\n", strings.TrimSpace(item.Topic), strings.TrimSpace(item.Content)))
		} else {
			b.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(item.Content)))
		}
	}
	return strings.TrimSpace(b.String())
}

// FormatBrandTastePrompt exports brand taste formatting for public callers and testing.
func (s *Server) FormatBrandTastePrompt(ctx context.Context) string {
	return s.formatBrandTastePrompt(ctx)
}

// formatTechnicalTastePrompt formats active Technical Taste preferences for Dev Terminal session.md runner injection.
func (s *Server) formatTechnicalTastePrompt(ctx context.Context) string {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	profile, err := s.getTasteProfile(ownerID)
	if err != nil || len(profile.TechnicalItems) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Technical Taste & Conventions\n")
	b.WriteString("Follow these technical standards, conventions, and operational rules during task execution:\n")
	for _, item := range profile.TechnicalItems {
		if strings.TrimSpace(item.Topic) != "" {
			b.WriteString(fmt.Sprintf("- [%s]: %s\n", strings.TrimSpace(item.Topic), strings.TrimSpace(item.Content)))
		} else {
			b.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(item.Content)))
		}
	}
	return strings.TrimSpace(b.String())
}

// FormatTechnicalTastePrompt exports technical taste formatting for public callers and testing.
func (s *Server) FormatTechnicalTastePrompt(ctx context.Context) string {
	return s.formatTechnicalTastePrompt(ctx)
}

// getTasteCandidates retrieves suggested/pending candidates for an owner.
func (s *Server) getTasteCandidates(owner string, filterStatus ...string) ([]TasteCandidate, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	s.taste.RLock()
	cached, ok := s.taste.candidates[ownerKey]
	s.taste.RUnlock()

	var all []TasteCandidate
	if ok {
		all = cached
	} else {
		s.taste.Lock()
		if cached, ok := s.taste.candidates[ownerKey]; ok {
			all = cached
		} else {
			items, err := readTasteCandidates(ownerKey)
			if err == nil && len(items) > 0 {
				s.taste.candidates[ownerKey] = items
				all = items
			} else {
				all = []TasteCandidate{}
				s.taste.candidates[ownerKey] = all
			}
		}
		s.taste.Unlock()
	}

	// Filter by status if requested.
	// If no filter given or empty, default to pending review (suggested or pending).
	if len(filterStatus) == 0 || filterStatus[0] == "" || filterStatus[0] == "pending" || filterStatus[0] == "suggested" {
		var filtered []TasteCandidate
		for _, c := range all {
			if c.Status == string(TasteCandidateStatusSuggested) || c.Status == string(TasteCandidateStatusPending) {
				filtered = append(filtered, c)
			}
		}
		if filtered == nil {
			filtered = []TasteCandidate{}
		}
		return filtered, nil
	}

	target := strings.ToLower(filterStatus[0])
	if target == "all" {
		return all, nil
	}

	var filtered []TasteCandidate
	for _, c := range all {
		if strings.ToLower(c.Status) == target {
			filtered = append(filtered, c)
		}
	}
	if filtered == nil {
		filtered = []TasteCandidate{}
	}
	return filtered, nil
}

// getAllTasteCandidates returns all candidates regardless of status.
func (s *Server) getAllTasteCandidates(owner string) ([]TasteCandidate, error) {
	return s.getTasteCandidates(owner, "all")
}

// getTasteCandidateByID finds a candidate by ID across all statuses.
func (s *Server) getTasteCandidateByID(owner, id string) (TasteCandidate, error) {
	candidates, err := s.getAllTasteCandidates(owner)
	if err != nil {
		return TasteCandidate{}, err
	}
	for _, c := range candidates {
		if c.ID == id {
			return c, nil
		}
	}
	return TasteCandidate{}, ErrTasteCandidateNotFound
}

// saveTasteCandidate stores a candidate with quota validation and deduplication.
func (s *Server) saveTasteCandidate(owner string, candidate TasteCandidate) (TasteCandidate, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	// Ensure candidates are loaded
	_, _ = s.getAllTasteCandidates(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.candidates[ownerKey]

	// Check safe quota limit for pending candidates
	pendingCount := 0
	for _, c := range current {
		if c.Status == string(TasteCandidateStatusSuggested) || c.Status == string(TasteCandidateStatusPending) {
			pendingCount++
		}
	}

	if pendingCount >= MaxTasteCandidates {
		return TasteCandidate{}, fmt.Errorf("Kapasitas maksimum %d kandidat saran telah tercapai", MaxTasteCandidates)
	}

	now := time.Now().UTC()
	if candidate.ID == "" {
		candidate.ID = db.GenerateUUID()
	}
	if candidate.Status == "" {
		candidate.Status = string(TasteCandidateStatusSuggested)
	}
	if candidate.Text == "" && candidate.Content != "" {
		candidate.Text = candidate.Content
	}
	if candidate.Content == "" && candidate.Text != "" {
		candidate.Content = candidate.Text
	}
	if candidate.Reason == "" && candidate.Rationale != "" {
		candidate.Reason = candidate.Rationale
	}
	if candidate.Rationale == "" && candidate.Reason != "" {
		candidate.Rationale = candidate.Reason
	}
	candidate.CreatedAt = now
	candidate.UpdatedAt = now

	// Deduplication check: check against existing candidates in store
	normNew := normalizeForDeduplication(candidate.Text)
	for _, c := range current {
		if normalizeForDeduplication(c.Text) == normNew || normalizeForDeduplication(c.Content) == normNew {
			return c, ErrTasteCandidateAlreadyExists
		}
	}

	updated := append(current, candidate)
	// If total candidates exceed MaxTasteCandidates * 2, prune oldest dismissed candidates
	if len(updated) > MaxTasteCandidates*2 {
		pruned := make([]TasteCandidate, 0, len(updated))
		for _, c := range updated {
			if c.Status != string(TasteCandidateStatusDismissed) {
				pruned = append(pruned, c)
			}
		}
		if len(pruned) < len(updated) {
			updated = pruned
		}
	}

	s.taste.candidates[ownerKey] = updated
	_ = writeTasteCandidates(ownerKey, updated)

	return candidate, nil
}

// acceptTasteCandidate accepts a suggested candidate, promoting it to an active TasteItem in profile.
func (s *Server) acceptTasteCandidate(owner, id string, overrideText, overrideTopic, overrideCategory *string) (TasteCandidate, TasteItem, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	_, _ = s.getTasteProfile(ownerKey)
	_, _ = s.getAllTasteCandidates(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.candidates[ownerKey]
	foundIdx := -1
	for i, c := range current {
		if c.ID == id {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return TasteCandidate{}, TasteItem{}, ErrTasteCandidateNotFound
	}

	candidate := current[foundIdx]

	cat := candidate.Category
	if overrideCategory != nil && strings.TrimSpace(*overrideCategory) != "" {
		normCat, err := normalizeCategory(*overrideCategory)
		if err != nil {
			return TasteCandidate{}, TasteItem{}, err
		}
		cat = normCat
	}
	topic := candidate.Topic
	if overrideTopic != nil {
		topic = strings.TrimSpace(*overrideTopic)
	}
	content := candidate.Text
	if overrideText != nil && strings.TrimSpace(*overrideText) != "" {
		content = strings.TrimSpace(*overrideText)
	}

	// Validate capacity in active taste profile
	profileCurrent := s.taste.profiles[ownerKey]
	catCount := 0
	for _, it := range profileCurrent {
		if it.Category == cat {
			catCount++
		}
	}
	if catCount >= MaxTasteItemsPerCategory {
		return TasteCandidate{}, TasteItem{}, fmt.Errorf("Kapasitas maksimum %d butir untuk kategori '%s' telah tercapai", MaxTasteItemsPerCategory, cat)
	}

	now := time.Now().UTC()
	item := TasteItem{
		ID:        db.GenerateUUID(),
		Category:  cat,
		Topic:     topic,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Add item to active profile
	updatedProfile := append(profileCurrent, item)
	s.taste.profiles[ownerKey] = updatedProfile
	_ = writeTastePreferences(ownerKey, updatedProfile)

	// Mark candidate as accepted
	candidate.Category = cat
	candidate.Topic = topic
	candidate.Text = content
	candidate.Content = content
	candidate.Status = string(TasteCandidateStatusAccepted)
	candidate.UpdatedAt = now
	current[foundIdx] = candidate
	s.taste.candidates[ownerKey] = current
	_ = writeTasteCandidates(ownerKey, current)

	return candidate, item, nil
}

// dismissTasteCandidate ignores/dismisses a suggested preference candidate.
func (s *Server) dismissTasteCandidate(owner, id string) (TasteCandidate, error) {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	_, _ = s.getAllTasteCandidates(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.candidates[ownerKey]
	foundIdx := -1
	for i, c := range current {
		if c.ID == id {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return TasteCandidate{}, ErrTasteCandidateNotFound
	}

	candidate := current[foundIdx]
	candidate.Status = string(TasteCandidateStatusDismissed)
	candidate.UpdatedAt = time.Now().UTC()
	current[foundIdx] = candidate
	s.taste.candidates[ownerKey] = current
	_ = writeTasteCandidates(ownerKey, current)

	return candidate, nil
}

// deleteTasteCandidate permanently removes a candidate.
func (s *Server) deleteTasteCandidate(owner, id string) error {
	if s.taste == nil {
		s.taste = newTasteStore()
	}
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	_, _ = s.getAllTasteCandidates(ownerKey)

	s.taste.Lock()
	defer s.taste.Unlock()

	current := s.taste.candidates[ownerKey]
	foundIdx := -1
	for i, c := range current {
		if c.ID == id {
			foundIdx = i
			break
		}
	}
	if foundIdx == -1 {
		return ErrTasteCandidateNotFound
	}

	updated := append(current[:foundIdx], current[foundIdx+1:]...)
	s.taste.candidates[ownerKey] = updated
	_ = writeTasteCandidates(ownerKey, updated)

	return nil
}
