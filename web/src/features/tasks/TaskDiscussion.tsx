import { useEffect, useState } from 'react'
import { apiFetch } from '../../api/client'
interface Comment { id: string; activity_type: string; author: string; message: string; created_at: string }
export function TaskDiscussion({ taskId }: { taskId: string }) {
  const [comments, setComments] = useState<Comment[]>([])
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const load = async () => {
    const result = await apiFetch<{ activities: Comment[] }>(`/api/v1/tasks/${taskId}`)
    setComments((result.activities || []).filter(activity => activity.activity_type === 'comment'))
  }
  useEffect(() => {
    load().catch(() => setError('Discussion could not be loaded.'))
    const poll = () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      load().catch(() => {})
    }
    const timer = setInterval(poll, 5000)
    const onResume = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'visible') {
        load().catch(() => {})
      }
    }
    document.addEventListener('visibilitychange', onResume)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onResume)
    }
  }, [taskId])
  const send = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!message.trim()) return
    setBusy(true); setError('')
    try { await apiFetch(`/api/v1/tasks/${taskId}/comments`, { method: 'POST', body: JSON.stringify({ message: message.trim() }) }); setMessage(''); await load() }
    catch { setError('Comment was not saved. Please retry.') }
    finally { setBusy(false) }
  }
  return <div className="space-y-4">
    <h3 className="font-semibold text-sm">Discussion</h3>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {comments.length === 0 && <p className="text-sm text-muted-foreground">No comments yet.</p>}
    {comments.map(comment => <article key={comment.id} className="rounded border border-border p-3 text-sm"><header className="mb-2 text-xs text-muted-foreground">{comment.author} · {new Date(comment.created_at).toLocaleString('en-US')}</header><p className="whitespace-pre-wrap break-words">{comment.message}</p></article>)}
    <form onSubmit={send} className="space-y-2"><label htmlFor="task-comment" className="text-sm">Add a comment</label><textarea id="task-comment" value={message} onChange={event => setMessage(event.target.value)} className="w-full rounded border border-input bg-background p-3 text-sm" rows={3} /><button disabled={busy || !message.trim()} className="rounded bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-50">{busy ? 'Saving…' : 'Send comment'}</button></form>
  </div>
}
