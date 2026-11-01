import type { Task } from '../../api/types'

export interface TaskReviewEnvelope {
  summary: string
  draft: string
  evidence: string
  requires_deploy: boolean
}

export interface ParsedEnvelopeResult {
  envelope: TaskReviewEnvelope | null
  summary: string
  draft: string
  evidence: string
  requiresDeploy: boolean
  isFromEnvelope: boolean
}

export function sanitizeJargon(text: string): string {
  if (!text) return ''
  let cleaned = text
  cleaned = cleaned.replace(/\b(?:runner\s+)?heartbeat\s+lost\b/gi, 'koneksi terputus')
  cleaned = cleaned.replace(/\b(?:and\s+)?lease\s+expired\b/gi, 'waktu sesi habis')
  cleaned = cleaned.replace(/\battempt\s*(?:#?\s*\d+)?\b/gi, '')
  cleaned = cleaned.replace(/\bdefinition\s+of\s+done\b/gi, '')
  cleaned = cleaned.replace(/\bdefinition-of-done\b/gi, '')
  cleaned = cleaned.replace(/\bdod\b/gi, '')
  cleaned = cleaned.replace(/\s{2,}/g, ' ').trim()
  return cleaned
}

/**
 * Parses the dual-channel output envelope from task summary, complete_note, or metadata.
 * Returns human-readable operator summary, visitor reply draft, technical evidence,
 * and deployment requirement status.
 */
export function parseTaskReviewEnvelope(
  task: Task | null | undefined,
  fallbackDraft?: string
): ParsedEnvelopeResult {
  if (!task) {
    return {
      envelope: null,
      summary: '',
      draft: fallbackDraft ? sanitizeJargon(fallbackDraft) : '',
      evidence: '',
      requiresDeploy: false,
      isFromEnvelope: false,
    }
  }

  const candidates = [
    task.summary,
    task.complete_note,
    task.description,
  ].filter(Boolean) as string[]

  let parsed: TaskReviewEnvelope | null = null

  for (const text of candidates) {
    if (!text || (!text.includes('{') && !text.includes('requires_deploy'))) continue

    // Direct JSON parse attempt
    try {
      const obj = JSON.parse(text)
      if (
        obj &&
        typeof obj === 'object' &&
        (obj.draft !== undefined || obj.summary !== undefined || obj.requires_deploy !== undefined)
      ) {
        parsed = {
          summary: String(obj.summary || ''),
          draft: String(obj.draft || ''),
          evidence: String(obj.evidence || ''),
          requires_deploy: Boolean(obj.requires_deploy),
        }
        break
      }
    } catch {
      // Try extracting nested JSON block
      const jsonMatch =
        text.match(/\{[\s\S]*"draft"[\s\S]*\}/) ||
        text.match(/\{[\s\S]*"requires_deploy"[\s\S]*\}/)
      if (jsonMatch) {
        try {
          const obj = JSON.parse(jsonMatch[0])
          if (obj && typeof obj === 'object') {
            parsed = {
              summary: String(obj.summary || ''),
              draft: String(obj.draft || ''),
              evidence: String(obj.evidence || ''),
              requires_deploy: Boolean(obj.requires_deploy),
            }
            break
          }
        } catch {
          // ignore parsing error
        }
      }
    }
  }

  // Check task.metadata
  if (!parsed && (task as any).metadata?.review_envelope) {
    const env = (task as any).metadata.review_envelope
    parsed = {
      summary: String(env.summary || ''),
      draft: String(env.draft || ''),
      evidence: String(env.evidence || ''),
      requires_deploy: Boolean(env.requires_deploy),
    }
  }

  const rawStatus = (task.status || '').toLowerCase().trim()
  const isDone = rawStatus === 'done'
  const isReview = rawStatus === 'review'

  // Extract operator summary
  let summary = parsed?.summary?.trim() || ''
  if (!summary) {
    const rawSummary = (task.summary || task.complete_note || task.description || '').trim()
    if (rawSummary.startsWith('{') && rawSummary.includes('"draft"')) {
      summary = isDone
        ? 'Instruksi tugas berhasil diselesaikan dengan baik.'
        : 'Tugas telah selesai diproses dan siap untuk direviu.'
    } else {
      summary = rawSummary
    }
  }

  if (!summary) {
    summary = isDone
      ? 'Instruksi tugas berhasil diselesaikan dengan baik.'
      : 'Tugas telah selesai diproses dan siap untuk direviu.'
  }

  // Extract visitor reply draft
  let draft =
    parsed?.draft?.trim() ||
    fallbackDraft?.trim() ||
    (task as any).reply_draft ||
    (task as any).metadata?.reply_draft ||
    ''

  if (!draft) {
    if (isDone) {
      draft = `Halo! Kami telah menyelesaikan pekerjaan terkait "${task.title}". Semuanya telah kami proses dengan baik. Silakan beri tahu kami jika ada detail lain yang Anda butuhkan.`
    } else if (isReview) {
      draft = `Halo! Terkait kebutuhan "${task.title}", kami telah menindaklanjutinya dan menyiapkan langkah terbaik untuk Anda. Silakan beri tahu kami jika ada detail yang ingin disesuaikan.`
    } else {
      draft = ''
    }
  }

  const evidence =
    parsed?.evidence?.trim() ||
    (task.complete_note && task.complete_note !== task.summary ? task.complete_note : '')

  const requiresDeploy = parsed ? parsed.requires_deploy : false

  return {
    envelope: parsed,
    summary: sanitizeJargon(summary),
    draft: sanitizeJargon(draft),
    evidence,
    requiresDeploy,
    isFromEnvelope: Boolean(parsed),
  }
}
