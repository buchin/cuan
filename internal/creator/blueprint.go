package creator

import (
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// Blueprint IDs
const (
	BlueprintBlog      = "blog"
	BlueprintYouTube   = "youtube"
	BlueprintOlshop    = "olshop"
	BlueprintAffiliate = "affiliate"
)

// Template represents a pre-configured creator project blueprint.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Icon        string          `json:"icon"`
	Room        RoomTemplate    `json:"room"`
	Tasks       []TaskTemplate  `json:"tasks"`
	Agent       AgentTemplate   `json:"agent"`
}

// RoomTemplate represents the initial collaboration room for the blueprint.
type RoomTemplate struct {
	Title string `json:"title"`
	Topic string `json:"topic"`
}

// TaskTemplate represents a structured pipeline task in the blueprint.
type TaskTemplate struct {
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Priority        string       `json:"priority"`
	DoDChecklist    []db.DoDItem `json:"dod_checklist"`
	Tags            []string     `json:"tags"`
	ExecutionStatus string       `json:"execution_status"`
}

// AgentTemplate represents the pre-configured AI persona bound to the blueprint.
type AgentTemplate struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	Avatar       string `json:"avatar"`
	SystemPrompt string `json:"system_prompt"`
	ModelID      string `json:"model_id"`
	Provider     string `json:"provider"`
}

// NormalizeBlueprintID maps synonyms and slugs to the canonical blueprint ID.
func NormalizeBlueprintID(id string) string {
	norm := strings.ToLower(strings.TrimSpace(id))
	norm = strings.ReplaceAll(norm, "-", "_")

	switch norm {
	case "blog", "blog_publisher", "blogger", "web_publisher":
		return BlueprintBlog
	case "youtube", "youtube_channel", "video", "video_creator", "yt":
		return BlueprintYouTube
	case "olshop", "online_shop", "digital_product", "ecommerce", "store", "shop":
		return BlueprintOlshop
	case "affiliate", "affiliate_site", "affiliate_marketing", "niche_affiliate":
		return BlueprintAffiliate
	default:
		return norm
	}
}

// DefaultTemplates returns the catalog of 4 canonical creator blueprints.
func DefaultTemplates() []Template {
	return []Template{
		// 1. Blog Publisher Blueprint
		{
			ID:          BlueprintBlog,
			Name:        "Blog Publisher",
			Description: "Automated content creation and SEO pipeline for niche bloggers and AdSense publishers.",
			Category:    "content",
			Icon:        "book-open",
			Room: RoomTemplate{
				Title: "SEO Research",
				Topic: "SEO keyword research, content planning, and pillar article drafts",
			},
			Agent: AgentTemplate{
				Name:   "SEO Specialist",
				Role:   "Search Engine Optimization Specialist & Content Strategist",
				Avatar: "https://api.dicebear.com/7.x/bottts/svg?seed=SEOSpecialist",
				SystemPrompt: "You are an expert SEO Specialist and Content Strategist. Your mission is to analyze high-intent keywords, conduct competitor SERP analysis, and produce comprehensive, high-ranking pillar content outlines and drafts following Google E-E-A-T guidelines.",
				ModelID:  db.DefaultAgentModelID,
				Provider: db.DefaultAgentProvider,
			},
			Tasks: []TaskTemplate{
				{
					Title:       "Keyword research",
					Description: "Perform keyword research, search intent mapping, and competitor SERP gap analysis for the target topic.",
					Priority:    "high",
					Tags:        []string{"seo", "research", "keywords"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Identified primary keyword and 5+ secondary LSI keywords", Completed: false},
						{Text: "Analyzed top 3 SERP competitor headings and word count", Completed: false},
						{Text: "Mapped user search intent (informational/commercial) and target audience", Completed: false},
						{Text: "Structured recommended H2/H3 article outline", Completed: false},
					},
				},
				{
					Title:       "Content drafting",
					Description: "Draft complete, comprehensive 1,500+ word article optimized for search engines and human engagement.",
					Priority:    "urgent",
					Tags:        []string{"content", "drafting", "article"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Engaging introduction with hook and clear thesis statement", Completed: false},
						{Text: "Comprehensive H2/H3 body sections covering all topical sub-questions", Completed: false},
						{Text: "Actionable takeaways, examples, or data points included", Completed: false},
						{Text: "Natural inclusion of primary and secondary keywords without stuffing", Completed: false},
						{Text: "Clear conclusion with compelling call-to-action (CTA)", Completed: false},
					},
				},
				{
					Title:       "On-page SEO",
					Description: "Optimize meta tags, URL slug, internal linking structure, image alt tags, and readability scores.",
					Priority:    "normal",
					Tags:        []string{"seo", "optimization", "metadata"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Meta title formulated under 60 characters with primary keyword", Completed: false},
						{Text: "Meta description under 155 characters with compelling click trigger", Completed: false},
						{Text: "SEO-friendly URL slug defined", Completed: false},
						{Text: "Image alt-text and schema markup recommendations documented", Completed: false},
					},
				},
			},
		},

		// 2. YouTube Channel Blueprint
		{
			ID:          BlueprintYouTube,
			Name:        "YouTube Channel",
			Description: "High-converting video production pipeline from CTR-optimized hooks to complete 10-minute scripts.",
			Category:    "video",
			Icon:        "video",
			Room: RoomTemplate{
				Title: "Channel Production",
				Topic: "Video ideation, high-CTR thumbnail hooks, full scripts, and descriptions",
			},
			Agent: AgentTemplate{
				Name:   "YouTube Scriptwriter",
				Role:   "YouTube Scriptwriter & Audience Retention Expert",
				Avatar: "https://api.dicebear.com/7.x/bottts/svg?seed=YouTubeScriptwriter",
				SystemPrompt: "You are a world-class YouTube Scriptwriter and Audience Retention Expert. You master high-CTR titles, opening 30-second retention hooks, engaging storytelling pacing, visual B-roll cues, and effective subscriber CTAs for 10-minute video formats.",
				ModelID:  db.DefaultAgentModelID,
				Provider: db.DefaultAgentProvider,
			},
			Tasks: []TaskTemplate{
				{
					Title:       "Ideation & CTR hook",
					Description: "Brainstorm 5 high-CTR title variations and write the critical first 30-second retention hook.",
					Priority:    "high",
					Tags:        []string{"youtube", "ideation", "hook"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "5 compelling title variations scored for curiosity gap and CTR", Completed: false},
						{Text: "30-second opening hook script designed to maximize initial viewer retention", Completed: false},
						{Text: "Visual thumbnail concept and text overlay planned", Completed: false},
						{Text: "Core video premise and promise clearly stated", Completed: false},
					},
				},
				{
					Title:       "10-min scriptwriting",
					Description: "Develop the full script for a 10-minute video complete with visual cues, B-roll notes, and pacing transitions.",
					Priority:    "urgent",
					Tags:        []string{"youtube", "script", "production"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Full voiceover script exceeding 1,200 words (approx. 8-10 minutes spoken)", Completed: false},
						{Text: "Visual B-roll and on-screen graphic cues documented in brackets", Completed: false},
						{Text: "Mid-roll engagement question or retention re-hook placed at 4-minute mark", Completed: false},
						{Text: "Smooth transition into the video conclusion", Completed: false},
					},
				},
				{
					Title:       "Description & tags",
					Description: "Write SEO-optimized YouTube description, timestamp chapters, playlist suggestions, and search tags.",
					Priority:    "normal",
					Tags:        []string{"youtube", "seo", "tags"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "SEO-optimized description with keywords in first 2 lines", Completed: false},
						{Text: "Timestamped video chapters (00:00 Intro, etc.) created", Completed: false},
						{Text: "Relevant hashtags (#3) and 15 targeted search tags generated", Completed: false},
						{Text: "End-screen video recommendation and subscription CTA included", Completed: false},
					},
				},
			},
		},

		// 3. Online Shop / Digital Product Blueprint
		{
			ID:          BlueprintOlshop,
			Name:        "Online Shop / Digital Product",
			Description: "Direct-response sales funnel pipeline with sales page copywriting, email nurture sequence, and objection handling.",
			Category:    "ecommerce",
			Icon:        "shopping-bag",
			Room: RoomTemplate{
				Title: "Sales Funnel",
				Topic: "Direct-response sales copy, 3-part email nurture, and objection handling",
			},
			Agent: AgentTemplate{
				Name:   "Conversion Copywriter",
				Role:   "Direct Response Copywriter & Funnel Strategist",
				Avatar: "https://api.dicebear.com/7.x/bottts/svg?seed=ConversionCopywriter",
				SystemPrompt: "You are an elite Direct Response Copywriter and Funnel Strategist. You specialize in persuasive sales pages, PAS/AIDA frameworks, multi-part automated email nurture sequences, and overcoming buyer objections to maximize conversion rates.",
				ModelID:  db.DefaultAgentModelID,
				Provider: db.DefaultAgentProvider,
			},
			Tasks: []TaskTemplate{
				{
					Title:       "Sales page copy",
					Description: "Write a high-converting sales page following the PAS (Problem-Agitate-Solution) framework.",
					Priority:    "urgent",
					Tags:        []string{"copywriting", "sales-page", "conversion"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Attention-grabbing headline and benefit-driven subheadline", Completed: false},
						{Text: "Pain point agitation detailing emotional and financial cost of problem", Completed: false},
						{Text: "Clear product presentation with feature-to-benefit breakdown", Completed: false},
						{Text: "Social proof placeholders, guarantee/risk reversal, and urgency CTA", Completed: false},
					},
				},
				{
					Title:       "3-part email nurture",
					Description: "Draft a 3-part automated email welcome & nurture sequence guiding leads to their first purchase.",
					Priority:    "high",
					Tags:        []string{"email", "nurture", "marketing"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Email 1: Welcome, deliver value, and set expectations", Completed: false},
						{Text: "Email 2: Case study, relatable transformation story, and empathy bridge", Completed: false},
						{Text: "Email 3: Direct pitch, exclusive discount/bonus, and deadline reminder", Completed: false},
						{Text: "High open-rate subject lines and preview text written for all 3 emails", Completed: false},
					},
				},
				{
					Title:       "FAQ & objections",
					Description: "Compile a comprehensive FAQ matrix addressing price, time commitment, guarantees, and competitor doubts.",
					Priority:    "normal",
					Tags:        []string{"copywriting", "faq", "objections"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Top 5-7 customer objections identified (price, complexity, support, trust)", Completed: false},
						{Text: "Persuasive rebuttal answers crafted for each objection", Completed: false},
						{Text: "Money-back guarantee and risk-free trial terms clearly articulated", Completed: false},
						{Text: "Formatted into accordion-ready Q&A markdown", Completed: false},
					},
				},
			},
		},

		// 4. Affiliate Site Blueprint
		{
			ID:          BlueprintAffiliate,
			Name:        "Affiliate Site",
			Description: "High-intent buyer comparison and review workflow with hands-on product analysis and compliance disclaimers.",
			Category:    "marketing",
			Icon:        "link",
			Room: RoomTemplate{
				Title: "Niche Affiliate",
				Topic: "Affiliate reviews, comparison matrix, buyer guides, and FTC disclaimers",
			},
			Agent: AgentTemplate{
				Name:   "Affiliate Strategist",
				Role:   "Affiliate Marketing Strategist & Reviewer",
				Avatar: "https://api.dicebear.com/7.x/bottts/svg?seed=AffiliateStrategist",
				SystemPrompt: "You are a seasoned Affiliate Marketing Strategist and Product Reviewer. You craft objective, trust-building product comparisons, pros & cons evaluations, clear buyer recommendations, and compliant affiliate disclosure statements.",
				ModelID:  db.DefaultAgentModelID,
				Provider: db.DefaultAgentProvider,
			},
			Tasks: []TaskTemplate{
				{
					Title:       "Product comparison matrix",
					Description: "Construct a comprehensive side-by-side comparison table between top products with ratings and feature criteria.",
					Priority:    "high",
					Tags:        []string{"affiliate", "comparison", "matrix"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Selected 3-5 competing products in the target niche", Completed: false},
						{Text: "Defined 5 key evaluation criteria (price, performance, ease of use, etc.)", Completed: false},
						{Text: "Structured Markdown comparison matrix with clear winner badge", Completed: false},
						{Text: "Quick verdict summary for budget, best overall, and premium options", Completed: false},
					},
				},
				{
					Title:       "Hands-on product review",
					Description: "Draft an in-depth, experiential product review highlighting unboxing, real performance, pros & cons, and value rating.",
					Priority:    "urgent",
					Tags:        []string{"affiliate", "review", "content"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Realistic hands-on testing scenarios and performance benchmarks", Completed: false},
						{Text: "Balanced Pros & Cons list with at least 3 genuine drawbacks", Completed: false},
						{Text: "Pricing value assessment and ideal target buyer persona", Completed: false},
						{Text: "Final verdict score out of 10 with recommendation breakdown", Completed: false},
					},
				},
				{
					Title:       "CTA & disclaimer",
					Description: "Craft high-CTR contextual affiliate call-to-action buttons, pricing tables, and FTC/regulatory compliance disclaimers.",
					Priority:    "normal",
					Tags:        []string{"affiliate", "cta", "compliance"},
					ExecutionStatus: "idle",
					DoDChecklist: []db.DoDItem{
						{Text: "Prominent, benefit-oriented CTA button copy (e.g. 'Check Lowest Price on Amazon')", Completed: false},
						{Text: "Legally compliant affiliate disclosure statement placed above the fold", Completed: false},
						{Text: "No-follow and sponsored tag guidelines noted for links", Completed: false},
						{Text: "Summary callout box with key purchase incentives", Completed: false},
					},
				},
			},
		},
	}
}

// LookupTemplate finds a template by ID or normalized synonym.
func LookupTemplate(id string) (*Template, bool) {
	normID := NormalizeBlueprintID(id)
	for _, t := range DefaultTemplates() {
		if t.ID == normID {
			// Return a copy
			cp := t
			return &cp, true
		}
	}
	return nil, false
}
