type Task = { id: string; source_message_id?: string; status?: string; release_status?: string }
type Message = { uuid?: string; id: number | string; author_type?: string; author_id?: string; is_internal?: boolean; content?: string; message?: string; work_update?: {task_id: string} }
type Request = { reply_id: string; message_id: string }

/** Bind each owner-visible task preview to a message that can actually render it. */
export function taskMessageBindings(tasks: Task[], messages: Message[], requests: Request[], customer: boolean) {
  const known = new Set(tasks.map(t => t.id))
  const sources = new Map<string, string[]>()
  for (const task of tasks) if (task.source_message_id) {
    const ids = sources.get(task.source_message_id) || []
    ids.push(task.id); sources.set(task.source_message_id, ids)
  }
  const replies = new Map(requests.map(q => [q.reply_id, q.message_id]))
  const result = new Map<string, string>()
  for (const message of messages) {
    const eligible = message.author_type === 'agent' && (!customer || message.is_internal === true) || message.author_type === 'system' && !customer
    const content = message.content || message.message || ''
    if (!eligible || message.author_id === 'cuan_work_review' || /^(?:I[’']m checking the latest work update|Review the result for \[|This update is no longer current)/.test(content)) continue
    const id = message.uuid || String(message.id)
    const ids = new Set(sources.get(id))
    if (message.work_update && known.has(message.work_update.task_id)) ids.add(message.work_update.task_id)
    for (const task of sources.get(replies.get(id) || '') || []) ids.add(task)
    for (const match of content.matchAll(/\/tasks\/([A-Za-z0-9_-]+)/g)) if (known.has(match[1])) ids.add(match[1])
    for (const task of ids) result.set(task, id)
  }
  return result
}

/** Background records retain their card/draft, while genuine decisions remain speech. */
export function isBackgroundWorkUpdate(message: {work_update?: {kind: string}; approval_plan?: unknown; prompt_choices?: unknown; action_card?: unknown; receipt_extraction?: unknown}) {
  return !!message.work_update && message.work_update.kind !== 'result' && !message.approval_plan && !message.prompt_choices && !message.action_card && !message.receipt_extraction
}

const RELEASE_PENDING_OR_FAILURE_RE = /Rilis hasil yang diperiksa belum tuntas|belum masuk ke aplikasi yang sedang berjalan|Cuan sedang menerapkannya melalui jalur rilis proyek|⚠️ Deploy production gagal|⚠️ Auto-apply tidak dapat menggabungkan|Verifikasi deployment production belum dikonfirmasi/i
const RELEASE_COMPLETION_RE = /telah selesai dan diverifikasi|sudah selesai dan diverifikasi|Pekerjaan ditandai selesai|telah diverifikasi aktif pada rilis live|Perubahan telah berhasil diterapkan/i

/**
 * Suppresses an older release-pending or release-failure notice once every
 * referenced task is done, live (not pending/deploying/failed), and followed
 * by a completion message for that task in the conversation.
 */
export function isStaleReleaseNoticeSuperseded(
  content: string,
  referencedTaskIds: string[],
  tasks: Task[],
  messages: Message[],
  msgIndex: number,
): boolean {
  if (!referencedTaskIds.length || !RELEASE_PENDING_OR_FAILURE_RE.test(content)) {
    return false
  }
  const allLiveDone = referencedTaskIds.every((id) => {
    const t = tasks.find((item) => item.id === id)
    return (
      t?.status === 'done' &&
      t.release_status !== 'pending' &&
      t.release_status !== 'deploying' &&
      t.release_status !== 'failed'
    )
  })
  if (!allLiveDone) return false
  return referencedTaskIds.every((id) =>
    messages.slice(msgIndex + 1).some((m) => {
      const text = m.content || m.message || ''
      return text.includes(`/tasks/${id}`) && RELEASE_COMPLETION_RE.test(text)
    }),
  )
}
