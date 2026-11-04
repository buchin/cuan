/** Task cards carry launch status. Preserve user quotes and actual failure text. */
export function withoutTaskLaunchAcknowledgement(body: string, hasTask: boolean, outgoing = false) {
  if (!hasTask || outgoing) return body
  return body.replace(/(^|\n\n)Work (?:queued|started)\. I[’']ll report progress and the result here\.(?=\n\n|$)/g, '$1').replace(/\n{3,}/g, '\n\n').trim()
}
