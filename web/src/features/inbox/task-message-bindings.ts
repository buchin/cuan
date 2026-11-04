type Task = { id: string; source_message_id?: string }
type Message = { uuid?: string; id: number | string; author_type?: string; author_id?: string; is_internal?: boolean; content?: string; message?: string }
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
    for (const task of sources.get(replies.get(id) || '') || []) ids.add(task)
    for (const match of content.matchAll(/\/tasks\/([A-Za-z0-9_-]+)/g)) if (known.has(match[1])) ids.add(match[1])
    for (const task of ids) result.set(task, id)
  }
  return result
}
