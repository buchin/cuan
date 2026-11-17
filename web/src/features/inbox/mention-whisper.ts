/**
 * Real-time mention detection and whisper mode helpers for customer room composer.
 * Adheres to Things 3 & Bear Notes calm visual standards and zero-clutter principles.
 */

export function extractMentionHandle(text: string): string {
  if (!text) return ''
  const parts = text.trim().split(/\s+/)
  for (const p of parts) {
    const clean = p.replace(/^[,!?:;"'()[\]{}]+|[,!?:;"'()[\]{}]+$/g, '')
    if (clean.startsWith('@') && clean.length > 1) {
      return clean
    }
  }
  return ''
}

export function isAgentMention(text: string): boolean {
  if (!text) return false
  const trimmed = text.trim()
  if (trimmed.startsWith('@')) return true
  const parts = trimmed.split(/\s+/)
  return parts.some((p) => {
    const clean = p.replace(/^[,!?:;"'()[\]{}]+|[,!?:;"'()[\]{}]+$/g, '')
    return clean.startsWith('@')
  })
}

export function isComposerWhisper(
  contactType: string | undefined,
  inputMessage: string,
): boolean {
  if (contactType !== 'customer') return false
  return isAgentMention(inputMessage)
}
