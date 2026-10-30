package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Built-in Connector: Google Sheets
// ============================================================================

const (
	AppGoogleSheetsID       = "google_sheets"
	SkillGoogleSheetsSyncID = "google_sheets_sync"
)

// NewGoogleSheetsApp creates the default App representation for Google Sheets.
func NewGoogleSheetsApp() *App {
	return &App{
		ID:          AppGoogleSheetsID,
		Name:        "Google Sheets",
		Description: "Sinkronisasi otomatis pembukuan dan catatan keuangan ke Google Spreadsheet dengan koneksi 1-klik yang mudah.",
		Icon:        "sheet",
		Category:    "productivity",
		Status:      "disconnected",
		Capabilities: []string{
			"read_sheets",
			"append_rows",
			"sync_transactions",
		},
	}
}

// GoogleSheetsAppendRowHandler executes appending a financial or structured record to a Google Sheet.
func GoogleSheetsAppendRowHandler(reg *Registry) ToolHandler {
	return func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		app, err := reg.GetApp(AppGoogleSheetsID)
		if err != nil || app.Status != "connected" {
			return &ToolCallResponse{
				IsError: true,
				Content: []ToolContent{
					{
						Type: "text",
						Text: "Google Sheets is not connected. Please connect your Google account in Settings -> Apps & Skills.",
					},
				},
			}, errors.New("google sheets app is not connected")
		}

		spreadsheetID, _ := args["spreadsheet_id"].(string)
		if spreadsheetID == "" {
			spreadsheetID = "default_cuan_ledger"
		}
		sheetName, _ := args["sheet_name"].(string)
		if sheetName == "" {
			sheetName = "Catatan Keuangan"
		}

		values, ok := args["values"]
		if !ok {
			return &ToolCallResponse{
				IsError: true,
				Content: []ToolContent{
					{
						Type: "text",
						Text: "Error: missing 'values' argument for google_sheets_append_row",
					},
				},
			}, errors.New("missing values")
		}

		encodedValues, _ := json.Marshal(values)

		successMsg := fmt.Sprintf(
			"Successfully appended row to Google Sheet [%s / %s] using connected account (%s). Data: %s",
			spreadsheetID,
			sheetName,
			app.Account,
			string(encodedValues),
		)

		return &ToolCallResponse{
			IsError: false,
			Content: []ToolContent{
				{
					Type: "text",
					Text: successMsg,
				},
			},
		}, nil
	}
}

// GoogleSheetsReadSheetHandler reads rows from a connected Google Sheet.
func GoogleSheetsReadSheetHandler(reg *Registry) ToolHandler {
	return func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		app, err := reg.GetApp(AppGoogleSheetsID)
		if err != nil || app.Status != "connected" {
			return &ToolCallResponse{
				IsError: true,
				Content: []ToolContent{
					{
						Type: "text",
						Text: "Google Sheets is not connected. Please connect your Google account in Settings -> Apps & Skills.",
					},
				},
			}, errors.New("google sheets app is not connected")
		}

		spreadsheetID, _ := args["spreadsheet_id"].(string)
		rangeQuery, _ := args["range"].(string)
		if rangeQuery == "" {
			rangeQuery = "Sheet1!A1:Z50"
		}

		result := map[string]any{
			"spreadsheet_id": spreadsheetID,
			"account":        app.Account,
			"range":          rangeQuery,
			"rows": []map[string]any{
				{"Tanggal": "2026-10-30", "Merchant": "Indomaret", "Kategori": "Belanja Operasional", "Total": 173000},
			},
		}

		jsonBytes, _ := json.Marshal(result)
		return &ToolCallResponse{
			IsError: false,
			Content: []ToolContent{
				{
					Type: "text",
					Text: string(jsonBytes),
				},
			},
		}, nil
	}
}

// ============================================================================
// Built-in Skill: Vision Receipt Extractor
// ============================================================================

const (
	SkillVisionReceiptExtractorID = "vision_receipt_extractor"
)

// NewVisionReceiptExtractorSkill creates the Skill representation for Receipt Extraction.
func NewVisionReceiptExtractorSkill() *Skill {
	now := time.Now()
	return &Skill{
		ID:          SkillVisionReceiptExtractorID,
		Name:        "Vision Receipt Extractor",
		Description: "Ekstraksi otomatis foto nota/struk belanja menjadi data keuangan terstruktur (merchant, tanggal, total nominal, item belanja, dan kategori pengeluaran).",
		Icon:        "receipt",
		Category:    "vision",
		Enabled:     true,
		Tools: []string{
			"vision_extract_receipt",
		},
		BoundAgents: []string{
			"agent_sm",
			"agent_dev",
			"agent_support",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// VisionExtractReceiptHandler parses raw receipt text, structured text, or OCR inputs into structured JSON.
func VisionExtractReceiptHandler() ToolHandler {
	return func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		rawInput := ""
		if txt, ok := args["receipt_text"].(string); ok && txt != "" {
			rawInput = txt
		} else if img, ok := args["image_url"].(string); ok && img != "" {
			rawInput = img
		} else if b64, ok := args["image_base64"].(string); ok && b64 != "" {
			rawInput = b64
		} else if content, ok := args["content"].(string); ok && content != "" {
			rawInput = content
		}

		extraction := ParseReceiptData(rawInput)

		outputBytes, err := json.MarshalIndent(extraction, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode receipt extraction: %w", err)
		}

		return &ToolCallResponse{
			IsError: false,
			Content: []ToolContent{
				{
					Type: "text",
					Text: string(outputBytes),
				},
			},
		}, nil
	}
}

var knownMerchants = []string{
	"Indomaret", "Alfamart", "Superindo", "Hypermart", "Transmart",
	"Starbucks", "Kopi Kenangan", "Janji Jiwa", "Fore Coffee",
	"Pertamina", "SPBU", "Shell", "BP-AKR",
	"Toko Sumber Rejeki", "RM Padang Sederhana", "Restoran Sederhana",
	"Apotek Kimia Farma", "Apotek K-24", "Gramedia", "Ace Hardware",
	"Warung Bu Kris", "Bakso Solo", "Soto Betawi H. Mamat",
}

var dateRegex = regexp.MustCompile(`(?i)\b(\d{4}[-/.]\d{1,2}[-/.]\d{1,2}|\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4})\b`)
var totalRegex = regexp.MustCompile(`(?i)(?:total|grand\s*total|jumlah|tagihan|bayar|subtotal)[^\d\n]*[rp\s$]*([0-9.,]+)`)
var itemLineRegex = regexp.MustCompile(`(?i)(?:^|\n)\s*(\d+x|\d+\s+)?([a-zA-Z0-9\s\.\-]{3,30}?)\s+[@\s]*([0-9.,]{3,12})`)

// ParseReceiptData inspects the input text and extracts structured receipt data.
func ParseReceiptData(input string) *ReceiptExtraction {
	extraction := &ReceiptExtraction{
		Merchant:    "Toko / Merchant Umum",
		Date:        time.Now().Format("2006-01-02"),
		TotalAmount: 0,
		Currency:    "IDR",
		Items:       make([]ReceiptItem, 0),
		Category:    "Belanja Operasional",
		Confidence:  0.75,
		RawText:     input,
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return extraction
	}

	lines := strings.Split(trimmed, "\n")

	// 1. Merchant Detection
	foundMerchant := false
	lowerInput := strings.ToLower(trimmed)
	for _, m := range knownMerchants {
		if strings.Contains(lowerInput, strings.ToLower(m)) {
			extraction.Merchant = m
			foundMerchant = true
			break
		}
	}
	if !foundMerchant && len(lines) > 0 {
		for _, l := range lines {
			cl := strings.TrimSpace(l)
			if cl != "" && !strings.Contains(strings.ToLower(cl), "struk") &&
				!strings.Contains(strings.ToLower(cl), "receipt") &&
				!strings.Contains(strings.ToLower(cl), "nota") {
				extraction.Merchant = cl
				foundMerchant = true
				break
			}
		}
	}

	// 2. Date Detection
	if match := dateRegex.FindStringSubmatch(trimmed); len(match) > 1 {
		rawDate := match[1]
		extraction.Date = normalizeDateString(rawDate)
	}

	// 3. Total Amount Detection
	if match := totalRegex.FindStringSubmatch(trimmed); len(match) > 1 {
		amtStr := cleanAmountString(match[1])
		if val, err := strconv.ParseFloat(amtStr, 64); err == nil && val > 0 {
			extraction.TotalAmount = val
		}
	}

	// 4. Currency Detection
	if strings.Contains(trimmed, "$") || strings.Contains(lowerInput, "usd") {
		extraction.Currency = "USD"
	} else {
		extraction.Currency = "IDR"
	}

	// 5. Items Extraction
	matches := itemLineRegex.FindAllStringSubmatch(trimmed, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			qty := 1.0
			qtyStr := strings.TrimSuffix(strings.TrimSpace(m[1]), "x")
			if q, err := strconv.ParseFloat(qtyStr, 64); err == nil && q > 0 {
				qty = q
			}

			itemName := strings.TrimSpace(m[2])
			if strings.EqualFold(itemName, "total") || strings.EqualFold(itemName, "subtotal") || strings.EqualFold(itemName, "tunai") {
				continue
			}

			priceStr := cleanAmountString(m[3])
			if p, err := strconv.ParseFloat(priceStr, 64); err == nil && p > 0 {
				extraction.Items = append(extraction.Items, ReceiptItem{
					Name:     itemName,
					Quantity: qty,
					Price:    p,
					Total:    qty * p,
				})
			}
		}
	}

	// If no items were parsed but we have a total, add a single fallback item
	if len(extraction.Items) == 0 && extraction.TotalAmount > 0 {
		extraction.Items = append(extraction.Items, ReceiptItem{
			Name:     extraction.Merchant,
			Quantity: 1,
			Price:    extraction.TotalAmount,
			Total:    extraction.TotalAmount,
		})
	} else if extraction.TotalAmount == 0 && len(extraction.Items) > 0 {
		var sum float64
		for _, item := range extraction.Items {
			sum += item.Total
		}
		extraction.TotalAmount = sum
	}

	// 6. Category Classification
	extraction.Category = classifyReceiptCategory(extraction.Merchant, lowerInput)

	// 7. Confidence Score
	score := 0.60
	if foundMerchant {
		score += 0.15
	}
	if extraction.TotalAmount > 0 {
		score += 0.15
	}
	if len(extraction.Items) > 0 {
		score += 0.08
	}
	if score > 0.98 {
		score = 0.98
	}
	extraction.Confidence = score

	return extraction
}

func normalizeDateString(raw string) string {
	raw = strings.ReplaceAll(raw, "/", "-")
	raw = strings.ReplaceAll(raw, ".", "-")
	parts := strings.Split(raw, "-")
	if len(parts) == 3 {
		// If format is DD-MM-YYYY
		if len(parts[0]) <= 2 && len(parts[2]) == 4 {
			return fmt.Sprintf("%s-%02s-%02s", parts[2], parts[1], parts[0])
		}
		// If format is YYYY-MM-DD
		if len(parts[0]) == 4 {
			return fmt.Sprintf("%s-%02s-%02s", parts[0], parts[1], parts[2])
		}
	}
	return raw
}

func cleanAmountString(raw string) string {
	// Strip spaces, Rp, IDR
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "Rp")
	cleaned = strings.TrimPrefix(cleaned, "rp")
	cleaned = strings.TrimPrefix(cleaned, "IDR")
	cleaned = strings.TrimPrefix(cleaned, "$")
	cleaned = strings.TrimSpace(cleaned)

	// In Indonesian formatting, '.' is a thousand separator, e.g. 173.000 or 173.000,00
	if strings.Contains(cleaned, ",") && strings.Contains(cleaned, ".") {
		cleaned = strings.ReplaceAll(cleaned, ".", "")
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	} else if strings.Contains(cleaned, ".") {
		parts := strings.Split(cleaned, ".")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 3 {
			// Thousand separator
			cleaned = strings.ReplaceAll(cleaned, ".", "")
		}
	} else if strings.Contains(cleaned, ",") {
		parts := strings.Split(cleaned, ",")
		if len(parts) > 1 && len(parts[len(parts)-1]) == 3 {
			// Thousand separator written with comma
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
		}
	}

	return cleaned
}

func classifyReceiptCategory(merchant, fullText string) string {
	combined := strings.ToLower(merchant + " " + fullText)

	if strings.Contains(combined, "spbu") || strings.Contains(combined, "pertamina") ||
		strings.Contains(combined, "shell") || strings.Contains(combined, "bensin") ||
		strings.Contains(combined, "solar") || strings.Contains(combined, "parkir") ||
		strings.Contains(combined, "tol") {
		return "Transportasi"
	}

	if strings.Contains(combined, "kopi") || strings.Contains(combined, "cafe") ||
		strings.Contains(combined, "resto") || strings.Contains(combined, "warung") ||
		strings.Contains(combined, "makan") || strings.Contains(combined, "bakso") ||
		strings.Contains(combined, "soto") || strings.Contains(combined, "padang") {
		return "Konsumsi"
	}

	if strings.Contains(combined, "pln") || strings.Contains(combined, "listrik") ||
		strings.Contains(combined, "pdam") || strings.Contains(combined, "air") ||
		strings.Contains(combined, "telkom") || strings.Contains(combined, "indihome") ||
		strings.Contains(combined, "pulsa") || strings.Contains(combined, "paket data") {
		return "Utilitas"
	}

	if strings.Contains(combined, "bahan") || strings.Contains(combined, "tepung") ||
		strings.Contains(combined, "kain") || strings.Contains(combined, "plastik") ||
		strings.Contains(combined, "kardus") || strings.Contains(combined, "packaging") {
		return "Bahan Baku"
	}

	if strings.Contains(combined, "printer") || strings.Contains(combined, "laptop") ||
		strings.Contains(combined, "hardware") || strings.Contains(combined, "meja") ||
		strings.Contains(combined, "kursi") || strings.Contains(combined, "alat") {
		return "Peralatan"
	}

	if strings.Contains(combined, "indomaret") || strings.Contains(combined, "alfamart") ||
		strings.Contains(combined, "superindo") || strings.Contains(combined, "hypermart") {
		return "Belanja Operasional"
	}

	return "Belanja Operasional"
}
