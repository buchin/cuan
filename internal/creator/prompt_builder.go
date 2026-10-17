package creator

import (
	"context"
	"fmt"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// Pipeline types for creator task execution
const (
	PipelineBlog      = "blog"
	PipelineYouTube   = "youtube"
	PipelineOlshop    = "olshop"
	PipelineAffiliate = "affiliate"
	PipelineGeneral   = "general"
)

// DetectPipeline classifies a task into a creator pipeline based on project type, title, description, and tags.
func DetectPipeline(taskTitle, taskDescription string, tags []string, projectType string) string {
	normType := NormalizeBlueprintID(projectType)
	switch normType {
	case BlueprintBlog:
		return PipelineBlog
	case BlueprintYouTube:
		return PipelineYouTube
	case BlueprintOlshop:
		return PipelineOlshop
	case BlueprintAffiliate:
		return PipelineAffiliate
	}

	// If projectType is empty or generic, inspect task tags, title, and description
	lowerCombined := strings.ToLower(taskTitle + " " + taskDescription + " " + strings.Join(tags, " "))

	// Check YouTube patterns
	if strings.Contains(lowerCombined, "youtube") ||
		strings.Contains(lowerCombined, "scriptwriting") ||
		strings.Contains(lowerCombined, "ctr hook") ||
		strings.Contains(lowerCombined, "retention hook") ||
		strings.Contains(lowerCombined, "video") {
		return PipelineYouTube
	}

	// Check Blog patterns
	if strings.Contains(lowerCombined, "keyword research") ||
		strings.Contains(lowerCombined, "content drafting") ||
		strings.Contains(lowerCombined, "on-page seo") ||
		strings.Contains(lowerCombined, "blog") ||
		strings.Contains(lowerCombined, "article") ||
		strings.Contains(lowerCombined, "lsi") {
		return PipelineBlog
	}

	// Check Olshop patterns
	if strings.Contains(lowerCombined, "sales page") ||
		strings.Contains(lowerCombined, "email nurture") ||
		strings.Contains(lowerCombined, "objections") ||
		strings.Contains(lowerCombined, "olshop") ||
		strings.Contains(lowerCombined, "funnel") ||
		strings.Contains(lowerCombined, "ecommerce") {
		return PipelineOlshop
	}

	// Check Affiliate patterns
	if strings.Contains(lowerCombined, "comparison matrix") ||
		strings.Contains(lowerCombined, "product review") ||
		strings.Contains(lowerCombined, "affiliate") ||
		strings.Contains(lowerCombined, "disclaimer") ||
		strings.Contains(lowerCombined, "pros & cons") {
		return PipelineAffiliate
	}

	return PipelineGeneral
}

// BuildCreatorPrompt crafts a specialized structured prompt for the LLM runner based on the creator pipeline and task DoD.
func BuildCreatorPrompt(ctx context.Context, task *db.ProjectTask, agentPersona *db.Agent, extraInstructions string, projectType ...string) (string, string) {
	projType := ""
	if len(projectType) > 0 {
		projType = projectType[0]
	}

	pipeline := DetectPipeline(task.Title, task.Description, task.Tags, projType)

	// 1. Build System Instruction
	var systemPrompt strings.Builder
	if agentPersona != nil && strings.TrimSpace(agentPersona.SystemPrompt) != "" {
		systemPrompt.WriteString(strings.TrimSpace(agentPersona.SystemPrompt))
		systemPrompt.WriteString("\n\n")
	} else {
		systemPrompt.WriteString("You are an elite autonomous AI Creator Specialist in Cuan Studio.\n\n")
	}

	systemPrompt.WriteString("CORE EXECUTION DIRECTIVES:\n")
	systemPrompt.WriteString("1. Produce complete, production-ready deliverables formatted in clean, professional GitHub-Flavored Markdown.\n")
	systemPrompt.WriteString("2. Strictly fulfill and satisfy every single item in the Definition of Done (DoD) checklist provided.\n")
	systemPrompt.WriteString("3. Maintain high topical authority, practical depth, and natural human engagement (Google E-E-A-T and conversion standards).\n")
	systemPrompt.WriteString("4. Do NOT output conversational filler or preamble (such as 'Sure, here is your...', 'As an AI...'). Start directly with the deliverable H1 heading.\n")
	systemPrompt.WriteString("5. Ensure all data tables, visual cues, outlines, or copy snippets are thoroughly fleshed out with no placeholder 'TBD' or truncated sections.\n")

	// 2. Build User Prompt
	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("# TASK EXECUTION SPECIFICATION: %s\n\n", strings.ToUpper(task.Title)))
	userPrompt.WriteString(fmt.Sprintf("**Pipeline Category:** %s\n", strings.ToUpper(pipeline)))
	userPrompt.WriteString(fmt.Sprintf("**Priority:** %s\n", task.Priority))
	if len(task.Tags) > 0 {
		userPrompt.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(task.Tags, ", ")))
	}
	userPrompt.WriteString(fmt.Sprintf("**Task Description:**\n%s\n\n", task.Description))

	// Pipeline-Specific Guidelines
	userPrompt.WriteString("## PIPELINE GUIDELINES & DELIVERABLE STRUCTURE:\n")
	switch pipeline {
	case PipelineBlog:
		userPrompt.WriteString("- Format output as a complete, rankable SEO asset or comprehensive research dossier.\n")
		userPrompt.WriteString("- If keyword research: include target primary keyword, 5+ secondary LSI keywords with search intent and difficulty, competitor SERP gap breakdown, and H2/H3 outline.\n")
		userPrompt.WriteString("- If content drafting: craft a comprehensive 1,500+ word pillar article with engaging hook, structured H2/H3 sections, actionable takeaways/data, natural keyword density, and clear CTA.\n")
		userPrompt.WriteString("- If on-page SEO: provide meta title (<60 chars), meta description (<155 chars), URL slug, image alt-text, and JSON-LD schema recommendations.\n")

	case PipelineYouTube:
		userPrompt.WriteString("- Format output for high viewer retention and CTR optimization.\n")
		userPrompt.WriteString("- If ideation & hook: craft 5 high-CTR curiosity-gap titles (scored 1-10), a punchy 30-second opening hook script, and visual thumbnail concept & text overlay.\n")
		userPrompt.WriteString("- If scriptwriting: produce a full 10-minute voiceover script (>1,200 spoken words) with explicit bracketed [Visual: ... / B-Roll: ... / Graphic: ...] cues and a 4-minute mid-roll engagement hook.\n")
		userPrompt.WriteString("- If description & tags: include first 2 lines hook, timestamped chapters (00:00 Intro, etc.), 3 hashtags, and 15 targeted search tags.\n")

	case PipelineOlshop:
		userPrompt.WriteString("- Format output using direct-response conversion copywriting principles.\n")
		userPrompt.WriteString("- If sales page copy: apply PAS (Problem - Agitate - Solution) framework, compelling headline & subheadline, deep emotional pain agitation, product benefits, social proof placeholders, risk reversal guarantee, and urgency CTA.\n")
		userPrompt.WriteString("- If email nurture: write a complete 3-part automated email sequence (Welcome, Relatable Transformation Story, Direct Pitch) with high open-rate subject lines and preview texts.\n")
		userPrompt.WriteString("- If FAQ & objections: address top 5-7 customer objections (price, time, trust, complexity) with persuasive rebuttals in accordion-ready Q&A markdown.\n")

	case PipelineAffiliate:
		userPrompt.WriteString("- Format output to build high buyer trust and transparent compliance.\n")
		userPrompt.WriteString("- If comparison matrix: construct a side-by-side Markdown comparison table covering 3-5 competing products across 5 criteria, with winner badges and budget/premium verdicts.\n")
		userPrompt.WriteString("- If product review: provide realistic hands-on experiential testing narrative, balanced Pros & Cons (at least 3 genuine drawbacks), and overall rating out of 10.\n")
		userPrompt.WriteString("- If CTA & disclaimer: include prominent benefit-driven CTA button copy, FTC-compliant affiliate disclosure above the fold, and rel='nofollow sponsored' link tagging rules.\n")

	default:
		userPrompt.WriteString("- Produce complete, detailed deliverables directly addressing all requirements in the task description and DoD checklist.\n")
	}
	userPrompt.WriteString("\n")

	// Definition of Done (DoD) Checklist
	userPrompt.WriteString("## MANDATORY DEFINITION OF DONE (DoD) CHECKLIST:\n")
	userPrompt.WriteString("Your generated content MUST explicitly fulfill every single item below:\n")
	if len(task.DoDChecklist) > 0 {
		for i, item := range task.DoDChecklist {
			userPrompt.WriteString(fmt.Sprintf("%d. [ ] %s\n", i+1, item.Text))
		}
	} else {
		userPrompt.WriteString("1. [ ] Complete deliverables matching all requirements in description.\n")
		userPrompt.WriteString("2. [ ] Professional Markdown formatting with actionable content.\n")
	}
	userPrompt.WriteString("\n")

	// Extra Instructions from User
	if strings.TrimSpace(extraInstructions) != "" {
		userPrompt.WriteString("## ADDITIONAL USER INSTRUCTIONS:\n")
		userPrompt.WriteString(strings.TrimSpace(extraInstructions))
		userPrompt.WriteString("\n\n")
	}

	userPrompt.WriteString("Now generate the full, comprehensive Markdown deliverable following all directives above.\n")

	return systemPrompt.String(), userPrompt.String()
}
