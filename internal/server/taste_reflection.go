package server

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

// ReflectTasteRequest defines the payload for triggering reflection extraction.
type ReflectTasteRequest struct {
	Text        string           `json:"text,omitempty"`
	Content     string           `json:"content,omitempty"`
	Messages    []ChatMessageDTO `json:"messages,omitempty"`
	RawMessages []string         `json:"raw_messages,omitempty"`
	RoomID      string           `json:"room_id,omitempty"`
	Category    string           `json:"category,omitempty"`
	AutoApprove *bool            `json:"auto_approve,omitempty"`
}

// ReflectTasteResponse defines the JSON response from POST /api/v1/taste/reflect.
type ReflectTasteResponse struct {
	Message      string           `json:"message"`
	Candidates   []TasteCandidate `json:"candidates"`
	Extracted    []TasteCandidate `json:"extracted"`
	AutoApproved []TasteCandidate `json:"auto_approved,omitempty"`
	Count        int              `json:"count"`
	Total        int              `json:"total"`
}

var (
	// Regex triggers for user preferences and corrections in Indonesian & English
	preferencePatterns = []*regexp.Regexp{
		// Explicit correction / preference prefix: "Koreksi: ...", "Aturan baru: ...", "Preferensi: ..."
		regexp.MustCompile(`(?i)(?:^|[\n\.\!\?])\s*(?:koreksi|perbaikan|catatan\s+gaya|aturan\s+(?:baru|kode|teknis|komunikasi)|preferensi(?:\s+baru|\s+brand|\s+teknis)?)\s*[:\-]\s*([^\n\.\!\?]{8,250})`),
		// "Tolong mulai sekarang gunakan ...", "Selalu gunakan ...", "Wajib gunakan ..."
		regexp.MustCompile(`(?i)(?:tolong\s+|mohon\s+|harap\s+)?(?:mulai\s+sekarang\s+)?((?:selalu\s+|harus\s+|wajib\s+)?(?:gunakan|pakai|terapkan)\s+[^\n\.\!\?]{8,250})`),
		// "Jangan pernah ...", "Jangan gunakan ...", "Hindari ..."
		regexp.MustCompile(`(?i)(?:tolong\s+|mohon\s+|harap\s+)?(?:mulai\s+sekarang\s+)?((?:jangan\s+(?:pernah\s+)?(?:gunakan|pakai|lakukan)?|hindari\s+|dilarang\s+|stop\s+)[^\n\.\!\?]{8,250})`),
		// "Saya lebih suka ...", "Kami prefer ..."
		regexp.MustCompile(`(?i)(?:saya|aku|ane|gue|kami)\s+(?:lebih\s+suka|prefer)\s+([^\n\.\!\?]{8,250})`),
		// "Gaya bahasa / nada bicara / format ... harus / wajib ..."
		regexp.MustCompile(`(?i)((?:gaya\s+bahasa|nada\s+bicara|tone\s+of\s+voice|format\s+(?:jawaban|penulisan|caption|konten))\s*(?:harus|wajib|sebaiknya|tolong)\s+[^\n\.\!\?]{8,250})`),
		// "Untuk backend / frontend / git / deploy selalu / wajib ..."
		regexp.MustCompile(`(?i)((?:untuk\s+(?:backend|frontend|api|kode|testing|git|deploy))\s*(?:selalu|harus|wajib|gunakan|pakai)\s+[^\n\.\!\?]{8,250})`),
		// English patterns: "always use ...", "never use ...", "prefer ...", "avoid ..."
		regexp.MustCompile(`(?i)(?:please\s+)?(?:from\s+now\s+on\s+)?((?:always\s+use|always\s+apply|never\s+use|prefer|avoid)\s+[^\n\.\!\?]{8,250})`),
		regexp.MustCompile(`(?i)(?:^|[\n\.\!\?])\s*(?:correction|preference|new\s+rule)\s*[:\-]\s*([^\n\.\!\?]{8,250})`),
	}

	techKeywords = []string{
		"backend", "frontend", "golang", "go", "react", "typescript", "javascript",
		"python", "node", "bun", "npm", "pnpm", "docker", "container", "deploy",
		"deployment", "git", "commit", "branch", "pr", "pull request", "test",
		"unit test", "api", "rest", "graphql", "sql", "database", "postgres",
		"redis", "orm", "handler", "endpoint", "ci/cd", "pipeline", "linux",
		"bash", "ssh", "curl", "caddy", "nginx", "port", "http", "architecture",
		"microservice", "runner", "terminal", "linter", "syntax",
	}

	brandKeywords = []string{
		"bahasa", "tone", "nada", "santai", "ramah", "formal", "gaul", "ringkas",
		"komunikasi", "caption", "pesan", "sapaan", "audiens", "pelanggan",
		"customer", "brand", "produk", "penulisan", "emoji", "panggilan",
		"voice", "copywriting", "konten", "marketing", "promosi", "solutif",
		"kalimat", "singkat", "bullet point", "bullet points", "faq", "kreator",
		"umkm", "humor", "bersahabat",
	}
)

// classifyCategory determines if a preference text belongs to 'brand' or 'technical' category.
func classifyCategory(text string, fallbackCategory ...string) string {
	if len(fallbackCategory) > 0 && strings.TrimSpace(fallbackCategory[0]) != "" {
		if norm, err := normalizeCategory(fallbackCategory[0]); err == nil {
			return norm
		}
	}

	lower := strings.ToLower(text)
	techScore := 0
	for _, kw := range techKeywords {
		if strings.Contains(lower, kw) {
			techScore++
		}
	}

	brandScore := 0
	for _, kw := range brandKeywords {
		if strings.Contains(lower, kw) {
			brandScore++
		}
	}

	if techScore > brandScore {
		return string(TasteCategoryTechnical)
	}
	return string(TasteCategoryBrand)
}

// inferTopic derives a human-friendly topic label from category and preference text.
func inferTopic(category, text string) string {
	lower := strings.ToLower(text)

	if category == string(TasteCategoryTechnical) {
		switch {
		case strings.Contains(lower, "git") || strings.Contains(lower, "branch") || strings.Contains(lower, "commit") || strings.Contains(lower, "deploy") || strings.Contains(lower, "pr"):
			return "Standar Git & Deployment"
		case strings.Contains(lower, "test") || strings.Contains(lower, "testing") || strings.Contains(lower, "unit test") || strings.Contains(lower, "verifikasi"):
			return "Targeted Testing & Verifikasi"
		case strings.Contains(lower, "backend") || strings.Contains(lower, "golang") || strings.Contains(lower, "go") || strings.Contains(lower, "api") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "rest"):
			return "Konvensi Backend & Tech Stack"
		case strings.Contains(lower, "frontend") || strings.Contains(lower, "react") || strings.Contains(lower, "tailwind") || strings.Contains(lower, "ui"):
			return "Konvensi Frontend & UI"
		case strings.Contains(lower, "docker") || strings.Contains(lower, "container") || strings.Contains(lower, "caddy") || strings.Contains(lower, "linux"):
			return "Infrastruktur & Lingkungan Kerja"
		default:
			return "Standar Teknis & Operasional"
		}
	}

	// Brand & Product category
	switch {
	case strings.Contains(lower, "santai") || strings.Contains(lower, "ramah") || strings.Contains(lower, "formal") || strings.Contains(lower, "tone") || strings.Contains(lower, "nada"):
		return "Tone & Gaya Bahasa"
	case strings.Contains(lower, "caption") || strings.Contains(lower, "copywriting") || strings.Contains(lower, "marketing") || strings.Contains(lower, "promosi") || strings.Contains(lower, "konten"):
		return "Format Konten & Copywriting"
	case strings.Contains(lower, "ringkas") || strings.Contains(lower, "singkat") || strings.Contains(lower, "bullet") || strings.Contains(lower, "poin"):
		return "Format Jawaban & Struktur"
	case strings.Contains(lower, "audiens") || strings.Contains(lower, "pelanggan") || strings.Contains(lower, "customer") || strings.Contains(lower, "sapaan") || strings.Contains(lower, "panggilan"):
		return "Gaya Komunikasi & Audiens"
	case strings.Contains(lower, "istilah") || strings.Contains(lower, "kata") || strings.Contains(lower, "kosakata"):
		return "Pedoman Istilah & Kosakata"
	default:
		return "Pedoman Brand & Komunikasi"
	}
}

// cleanExtractedText trims filler words and capitalizes the first character.
func cleanExtractedText(raw string) string {
	s := strings.TrimSpace(raw)
	// Strip trailing punctuation
	s = strings.TrimRight(s, ".!?,;:")

	// Strip leading filler
	lower := strings.ToLower(s)
	fillers := []string{
		"tolong ", "mohon ", "harap ", "eh ", "oh ya ", "mulai sekarang ",
		"ingat bahwa ", "catatan: ", "aturan: ", "koreksi: ", "preferensi: ",
	}
	for _, f := range fillers {
		if strings.HasPrefix(lower, f) {
			s = strings.TrimSpace(s[len(f):])
			lower = strings.ToLower(s)
		}
	}

	if s == "" {
		return ""
	}

	// Capitalize first letter
	r, size := utf8.DecodeRuneInString(s)
	firstUpper := strings.ToUpper(string(r))
	cleaned := firstUpper + s[size:]

	// Append dot if alphanumeric
	if !strings.HasSuffix(cleaned, ".") && !strings.HasSuffix(cleaned, "!") {
		cleaned += "."
	}

	return cleaned
}

// isDuplicateCandidate checks if candidate text is identical or substantially covered by active items or existing candidates.
func isDuplicateCandidate(norm string, seen map[string]bool, activeItems []TasteItem, existingCandidates []TasteCandidate, currentBatch ...TasteCandidate) bool {
	if norm == "" || seen[norm] {
		return true
	}
	for _, it := range activeItems {
		normActive := normalizeForDeduplication(it.Content)
		if normActive == "" {
			continue
		}
		if norm == normActive {
			return true
		}
		if strings.Contains(normActive, norm) && float64(len(norm))/float64(len(normActive)) >= 0.4 {
			return true
		}
		if strings.Contains(norm, normActive) && float64(len(normActive))/float64(len(norm)) >= 0.4 {
			return true
		}
	}
	for _, c := range existingCandidates {
		normCand := normalizeForDeduplication(c.Text)
		if normCand == "" {
			normCand = normalizeForDeduplication(c.Content)
		}
		if normCand == "" {
			continue
		}
		if norm == normCand {
			return true
		}
		if strings.Contains(normCand, norm) && float64(len(norm))/float64(len(normCand)) >= 0.4 {
			return true
		}
		if strings.Contains(norm, normCand) && float64(len(normCand))/float64(len(norm)) >= 0.4 {
			return true
		}
	}
	for _, c := range currentBatch {
		normCand := normalizeForDeduplication(c.Text)
		if normCand == "" {
			normCand = normalizeForDeduplication(c.Content)
		}
		if normCand == "" {
			continue
		}
		if norm == normCand {
			return true
		}
		if strings.Contains(normCand, norm) && float64(len(norm))/float64(len(normCand)) >= 0.4 {
			return true
		}
		if strings.Contains(norm, normCand) && float64(len(normCand))/float64(len(norm)) >= 0.4 {
			return true
		}
	}
	return false
}

// CalculateConfidence determines a realistic confidence score (0.0 - 1.0) based on phrasing patterns.
func CalculateConfidence(text, snippet string, isExplicitPrefix bool) float64 {
	lowerSnippet := strings.ToLower(snippet)
	lowerText := strings.ToLower(text)
	combined := lowerSnippet + " " + lowerText

	score := 0.75

	// 1. Explicit prefixes: "koreksi:", "perbaikan:", "aturan baru:", "preferensi:", "catatan gaya:", "correction:", "new rule:", "preference:"
	if isExplicitPrefix ||
		strings.Contains(lowerSnippet, "koreksi:") || strings.Contains(lowerSnippet, "perbaikan:") ||
		strings.Contains(lowerSnippet, "aturan baru:") || strings.Contains(lowerSnippet, "aturan:") ||
		strings.Contains(lowerSnippet, "preferensi:") || strings.Contains(lowerSnippet, "catatan gaya:") ||
		strings.Contains(lowerSnippet, "correction:") || strings.Contains(lowerSnippet, "new rule:") ||
		strings.Contains(lowerSnippet, "preference:") {
		score = 0.95
		if strings.Contains(combined, "wajib") || strings.Contains(combined, "harus") || strings.Contains(combined, "selalu") || strings.Contains(combined, "always") || strings.Contains(combined, "never") {
			score = 0.98
		}
	} else if strings.Contains(combined, "sebaiknya") || strings.Contains(combined, "mungkin") ||
		strings.Contains(combined, "usahakan") || strings.Contains(combined, "bisa") {
		// 2. Suggestive or softer directives: "sebaiknya", "mungkin", "bisa", "usahakan"
		score = 0.65
	} else if strings.Contains(combined, "wajib") || strings.Contains(combined, "harus") ||
		strings.Contains(combined, "selalu") || strings.Contains(combined, "jangan pernah") ||
		strings.Contains(combined, "dilarang") || strings.Contains(combined, "always") ||
		strings.Contains(combined, "never") {
		// 3. Strong mandatory directives: "wajib", "harus", "selalu", "jangan pernah", "dilarang", "always", "never"
		score = 0.90
	} else if strings.Contains(combined, "gunakan") || strings.Contains(combined, "pakai") ||
		strings.Contains(combined, "terapkan") || strings.Contains(combined, "hindari") ||
		strings.Contains(combined, "stop") || strings.Contains(combined, "gaya bahasa") ||
		strings.Contains(combined, "tone of voice") || strings.Contains(combined, "format") {
		// 4. Clear action directives: "gunakan", "pakai", "terapkan", "hindari", "stop", "gaya bahasa", "tone of voice", "format"
		score = 0.85
	} else if strings.Contains(combined, "lebih suka") || strings.Contains(combined, "prefer") {
		// 5. Stated preference: "lebih suka", "prefer"
		score = 0.80
	}

	return math.Round(score*100) / 100
}

// ExtractTasteCandidates scans text for preferences or corrections and extracts candidate items.
// It rigorously prevents duplicates against active items, existing candidates, and intra-batch items.
func ExtractTasteCandidates(text string, activeItems []TasteItem, existingCandidates []TasteCandidate, fallbackCategory ...string) []TasteCandidate {
	text = strings.TrimSpace(text)
	if text == "" {
		return []TasteCandidate{}
	}

	// Prepare deduplication lookup
	seen := make(map[string]bool)

	// Mark active items as already existing
	for _, it := range activeItems {
		norm := normalizeForDeduplication(it.Content)
		if norm != "" {
			seen[norm] = true
		}
	}

	// Mark existing candidates as already existing
	for _, c := range existingCandidates {
		normText := normalizeForDeduplication(c.Text)
		if normText != "" {
			seen[normText] = true
		}
		normContent := normalizeForDeduplication(c.Content)
		if normContent != "" {
			seen[normContent] = true
		}
	}

	var candidates []TasteCandidate
	now := time.Now().UTC()

	// 1. Try regex matching on preference patterns
	for idx, pattern := range preferencePatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			matchedSnippet := strings.TrimSpace(m[0])
			extractedText := cleanExtractedText(m[1])
			if utf8.RuneCountInString(extractedText) < 10 {
				continue
			}

			norm := normalizeForDeduplication(extractedText)
			if isDuplicateCandidate(norm, seen, activeItems, existingCandidates, candidates...) {
				continue // Skip duplicates
			}
			seen[norm] = true

			category := classifyCategory(extractedText, fallbackCategory...)
			topic := inferTopic(category, extractedText)
			reason := fmt.Sprintf("Terdeteksi dari preferensi/koreksi percakapan: \"%s\"", truncateString(matchedSnippet, 120))

			isExplicit := (idx == 0 || idx == len(preferencePatterns)-1)
			conf := CalculateConfidence(extractedText, matchedSnippet, isExplicit)

			cand := TasteCandidate{
				ID:            db.GenerateUUID(),
				Category:      category,
				Topic:         topic,
				Text:          extractedText,
				Content:       extractedText,
				SourceSnippet: truncateString(matchedSnippet, 250),
				Reason:        reason,
				Rationale:     reason,
				Status:        string(TasteCandidateStatusSuggested),
				Confidence:    conf,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			candidates = append(candidates, cand)
		}
	}

	// 2. Fallback: if no candidates extracted via regex and the text itself looks like a direct preference rule
	if len(candidates) == 0 {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			// Remove list bullets if present
			trimmedLine = strings.TrimLeft(trimmedLine, "-*1234567890. ")
			if utf8.RuneCountInString(trimmedLine) < 15 || utf8.RuneCountInString(trimmedLine) > 300 {
				continue
			}

			// Check if line contains preference or instruction keywords
			lower := strings.ToLower(trimmedLine)
			hasDirective := strings.Contains(lower, "gunakan") || strings.Contains(lower, "pakai") ||
				strings.Contains(lower, "jangan") || strings.Contains(lower, "hindari") ||
				strings.Contains(lower, "selalu") || strings.Contains(lower, "wajib") ||
				strings.Contains(lower, "prefer") || strings.Contains(lower, "lebih suka") ||
				strings.Contains(lower, "format") || strings.Contains(lower, "gaya bahasa") ||
				strings.Contains(lower, "aturan") || strings.Contains(lower, "koreksi") ||
				strings.Contains(lower, "always") || strings.Contains(lower, "never")

			if !hasDirective {
				continue
			}

			extractedText := cleanExtractedText(trimmedLine)
			norm := normalizeForDeduplication(extractedText)
			if isDuplicateCandidate(norm, seen, activeItems, existingCandidates) {
				continue
			}
			seen[norm] = true

			category := classifyCategory(extractedText, fallbackCategory...)
			topic := inferTopic(category, extractedText)
			reason := fmt.Sprintf("Terdeteksi dari instruksi pengguna: \"%s\"", truncateString(trimmedLine, 120))

			conf := CalculateConfidence(extractedText, trimmedLine, false)
			if conf > 0.82 {
				conf = 0.82
			}

			cand := TasteCandidate{
				ID:            db.GenerateUUID(),
				Category:      category,
				Topic:         topic,
				Text:          extractedText,
				Content:       extractedText,
				SourceSnippet: truncateString(trimmedLine, 250),
				Reason:        reason,
				Rationale:     reason,
				Status:        string(TasteCandidateStatusSuggested),
				Confidence:    conf,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			candidates = append(candidates, cand)
		}
	}

	if candidates == nil {
		return []TasteCandidate{}
	}
	return candidates
}

func truncateString(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-3]) + "..."
}

// reflectTaste processes incoming text/messages, extracts candidates, and stores them safely.
func (s *Server) reflectTaste(ctx context.Context, owner string, req ReflectTasteRequest) ([]TasteCandidate, error) {
	ownerKey := strings.TrimSpace(owner)
	if ownerKey == "" {
		ownerKey = "default"
	}

	// 1. Gather text sources
	var textSources []string

	if strings.TrimSpace(req.Text) != "" {
		textSources = append(textSources, strings.TrimSpace(req.Text))
	}
	if strings.TrimSpace(req.Content) != "" && strings.TrimSpace(req.Content) != strings.TrimSpace(req.Text) {
		textSources = append(textSources, strings.TrimSpace(req.Content))
	}
	for _, raw := range req.RawMessages {
		if strings.TrimSpace(raw) != "" {
			textSources = append(textSources, strings.TrimSpace(raw))
		}
	}
	for _, m := range req.Messages {
		msgText := strings.TrimSpace(m.Content)
		if msgText == "" {
			msgText = strings.TrimSpace(m.Message)
		}
		if msgText != "" {
			textSources = append(textSources, msgText)
		}
	}

	// Manual Settings reflection with no explicit source examines a bounded
	// selection of recent rooms, using only messages attributed to this member.
	rooms := []string{}
	if room := strings.TrimSpace(req.RoomID); room != "" {
		rooms = append(rooms, room)
	} else if len(textSources) == 0 && s.projectStudioRepo != nil && strings.TrimSpace(owner) != "" {
		recent, err := s.projectStudioRepo.ListAllRooms(ctx)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(recent, func(i, j int) bool { return recent[i].UpdatedAt.After(recent[j].UpdatedAt) })
		for i, room := range recent {
			if i >= 20 {
				break
			}
			rooms = append(rooms, room.ID)
		}
	}
	for _, room := range rooms {
		owned := map[string]bool{}
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			requests, err := store.ListInboxRequests(ctx, room)
			if err != nil {
				return nil, err
			}
			for _, request := range requests {
				if request.OwnerID == ownerKey {
					owned[request.MessageID] = true
				}
			}
		}
		if len(owned) == 0 {
			continue
		}
		messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
		if err != nil {
			return nil, err
		}
		count := 0
		for i := len(messages) - 1; i >= 0 && count < 100; i-- {
			m := messages[i]
			if m.AuthorType != "user" || m.AuthorID != "operator_user" || !owned[m.ID] {
				continue
			}
			text := operatorPreferenceText(m.Content)
			if durablePreferenceCue.MatchString(text) {
				textSources = append(textSources, agent.MaskSecrets(text, nil))
				count++
			}
		}
	}

	combinedText := strings.Join(textSources, "\n")
	if strings.TrimSpace(combinedText) == "" {
		return []TasteCandidate{}, nil
	}

	// 2. Fetch active items and existing candidates for deduplication
	profile, _ := s.getTasteProfile(ownerKey)
	existingCandidates, _ := s.getAllTasteCandidates(ownerKey)

	// 3. Extract candidate taste items
	extracted := ExtractTasteCandidates(combinedText, profile.Items, existingCandidates, req.Category)
	if len(extracted) == 0 {
		return []TasteCandidate{}, nil
	}

	// 4. Auto-Approval Evaluation & Candidate Saving
	cfg := s.getTasteConfig(ownerKey)
	autoApproveActive := cfg.AutoApproveEnabled
	if req.AutoApprove != nil {
		autoApproveActive = *req.AutoApprove
	}
	threshold := cfg.AutoApproveThreshold
	if threshold <= 0 {
		threshold = 0.80
	}

	brandCount := len(profile.BrandItems)
	techCount := len(profile.TechnicalItems)

	var saved []TasteCandidate
	now := time.Now().UTC()

	for _, cand := range extracted {
		cat := cand.Category
		currentCatCount := brandCount
		if cat == string(TasteCategoryTechnical) {
			currentCatCount = techCount
		}

		if autoApproveActive && cand.Confidence >= threshold && currentCatCount < MaxTasteItemsPerCategory {
			// Auto-approve: promote candidate to active taste item
			item := TasteItem{
				ID:             db.GenerateUUID(),
				Category:       cat,
				Topic:          cand.Topic,
				Content:        cand.Text,
				AutoApproved:   true,
				AutoApprovedAt: &now,
				CreatedAt:      now,
				UpdatedAt:      now,
			}

			s.taste.Lock()
			next := append(append([]TasteItem(nil), s.taste.profiles[ownerKey]...), item)
			if err := writeTastePreferences(ownerKey, next); err != nil {
				s.taste.Unlock()
				return saved, err
			}
			s.taste.profiles[ownerKey] = next
			s.taste.Unlock()

			if cat == string(TasteCategoryTechnical) {
				techCount++
			} else {
				brandCount++
			}

			cand.Status = string(TasteCandidateStatusAccepted)
			cand.UpdatedAt = now
			res, err := s.saveTasteCandidate(ownerKey, cand)
			if err == nil {
				saved = append(saved, res)
			}
		} else {
			// Save as suggested / pending candidate for manual review
			cand.Status = string(TasteCandidateStatusSuggested)
			res, err := s.saveTasteCandidate(ownerKey, cand)
			if err == nil {
				saved = append(saved, res)
			}
		}
	}

	if saved == nil {
		saved = []TasteCandidate{}
	}
	return saved, nil
}
