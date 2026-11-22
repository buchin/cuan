/**
 * Helpers for contact avatar monogram initials and anonymous visitor detection.
 * Aligns with Things 3 / Bear Notes zero cognitive fatigue and low visual noise.
 */

/**
 * Checks whether a contact name represents an anonymous visitor or generic placeholder.
 */
export function isAnonymousContact(name?: string): boolean {
  if (!name) return true
  const trimmed = name.trim().toLowerCase()
  if (!trimmed) return true

  // Standard anonymous placeholders
  if (
    trimmed.startsWith('visitor') ||
    trimmed.startsWith('pengunjung') ||
    trimmed.startsWith('guest') ||
    trimmed.startsWith('anon') ||
    trimmed === 'customer room' ||
    trimmed === 'agent room' ||
    trimmed.startsWith('room #')
  ) {
    return true
  }

  return false
}

/**
 * Extracts 1-2 uppercase monogram initials from a contact name.
 * Returns empty string if the contact is anonymous or has no valid alphabetic characters.
 *
 * Examples:
 * - "fahmi putra" -> "FP"
 * - "kisworo izar" -> "KI"
 * - "azizoke 2022-05-30 09:30:43" -> "AZ"
 * - "Website pelanggan" -> "WP"
 * - "Alice" -> "AL"
 * - "A" -> "A"
 * - "Visitor #12345" -> "" (anonymous fallback to icon)
 */
export function getContactInitials(name?: string): string {
  if (!name || isAnonymousContact(name)) return ''

  // Split words by whitespace
  const rawWords = name.trim().split(/\s+/)
  if (rawWords.length === 0) return ''

  // Filter for words that start with a letter (unicode-aware)
  // This discards purely numeric or timestamp parts like "2022-05-30" or "09:30:43"
  const letterWords = rawWords.filter((w) => /^\p{L}/u.test(w))

  if (letterWords.length === 0) {
    const allLetters = name.replace(/[^\p{L}]/gu, '')
    if (allLetters.length >= 2) return allLetters.slice(0, 2).toUpperCase()
    if (allLetters.length === 1) return allLetters.toUpperCase()
    return ''
  }

  if (letterWords.length === 1) {
    // Single word name (e.g. "azizoke" or "Alice"): take first 2 letters
    const letters = letterWords[0].replace(/[^\p{L}]/gu, '')
    if (letters.length >= 2) {
      return letters.slice(0, 2).toUpperCase()
    }
    return letters.toUpperCase()
  }

  // Multi-word name: first letter of first word + first letter of second word
  const firstLetter = letterWords[0].charAt(0).toUpperCase()
  const secondLetter = letterWords[1].charAt(0).toUpperCase()
  return `${firstLetter}${secondLetter}`
}
