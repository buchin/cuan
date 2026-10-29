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
)

// ErrTasteItemNotFound is returned when an item cannot be found in the taste store.
var ErrTasteItemNotFound = errors.New("taste item not found")

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
	profiles map[string][]TasteItem
}

func newTasteStore() *tasteStore {
	return &tasteStore{
		profiles: make(map[string][]TasteItem),
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
