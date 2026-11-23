/** Task cards carry launch status. Preserve user quotes and actual failure text. */
export function withoutTaskLaunchAcknowledgement(body: string, hasTask: boolean, outgoing = false) {
  if (!hasTask || outgoing) return body
  return body.replace(/(^|\n\n)Work (?:queued|started)\. I[’']ll report progress and the result here\.(?=\n\n|$)/g, '$1').replace(/\n{3,}/g, '\n\n').trim()
}

/**
 * Strip duplicate task summary from bubble text when a task card is present.
 * Keeps only the introductory conversational notice (e.g. "Hasil pekerjaan memenuhi kriteria yang disepakati.")
 * so the summary isn't repeated verbatim above the TaskCompletionCard.
 */
export function withoutDuplicateTaskSummary(body: string, hasTask: boolean, outgoing = false) {
  if (!hasTask || outgoing) return body

  const introPatterns = [
    /^Hasil pekerjaan memenuhi kriteria(?: yang disepakati)?\.?/i,
    /^Hasil pekerjaan telah diterima dan diverifikasi\.?/i,
    /^Hasil pekerjaan ini sudah diterima\.?/i,
    /^Hasil pekerjaan sudah ditinjau\.?/i,
    /^Pekerjaan ditandai selesai\.?/i,
    /^Pekerjaan telah berhasil diselesaikan(?: dan diverifikasi)?\.?/i,
    /^The work has been completed and verified\.?/i,
    /^The result has been accepted and verified\.?/i,
    /^Work is completed\.?/i,
  ]

  for (const pattern of introPatterns) {
    if (pattern.test(body)) {
      const paragraphs = body.split(/\n\n+/)
      if (paragraphs.length > 1) {
        return paragraphs[0].trim()
      }
    }
  }

  return body
}
