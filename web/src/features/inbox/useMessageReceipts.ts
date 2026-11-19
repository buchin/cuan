import { useEffect } from 'react'
import { apiFetch } from '../../api/client'
import type { ChatMessage } from '../../api/chat'

// Receipt acknowledgements are independent of the Inbox unread counter.
export function useMessageReceipts(room: string | undefined, enabled: boolean, messages: ChatMessage[]) {
  useEffect(() => {
    if (!room || !enabled) return
    let stopped = false
    let pending = false
    const delivered = new Set<string>()
    const read = new Set<string>()
    const visible = new Set<string>()
    const incoming = messages.filter(m => m.uuid && !m.is_internal && m.sender_type === 'visitor')
    for (const m of incoming) if (!m.delivered_at) delivered.add(m.uuid!)
    const flush = async () => {
      if (stopped || pending || (!delivered.size && !read.size)) return
      pending = true
      let succeeded = false
      const d = [...delivered].slice(0, 500), r = [...read].slice(0, 500)
      try {
        await apiFetch(`/api/v1/chat/conversations/${encodeURIComponent(room)}/receipts`, {
          method: 'POST', signal: AbortSignal.timeout(10000), body: JSON.stringify({ delivered_ids: d, read_ids: r }),
        })
        d.forEach(id => delivered.delete(id)); r.forEach(id => read.delete(id))
        succeeded = true
      } catch { /* Retry acknowledgements without changing message state locally. */ }
      finally {
        pending = false
        if (succeeded && !stopped && (delivered.size || read.size)) queueMicrotask(() => void flush())
      }
    }
    const acknowledgeVisible = () => {
      if (document.visibilityState !== 'visible') return
      for (const id of visible) read.add(id)
      void flush()
    }
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) {
        const id = (entry.target as HTMLElement).dataset.messageId!
        if (entry.isIntersecting && entry.intersectionRect.height > 0) visible.add(id)
        else visible.delete(id)
      }
      acknowledgeVisible()
    })
    for (const m of incoming) {
      if (m.read_confirmed_at) continue
      const el = document.querySelector(`[data-message-id="${CSS.escape(m.uuid!)}"]`)
      if (el) observer.observe(el)
    }
    window.addEventListener('focus', acknowledgeVisible)
    document.addEventListener('visibilitychange', acknowledgeVisible)
    void flush()
    const retry = window.setInterval(() => void flush(), 3500)
    return () => { stopped = true; observer.disconnect(); clearInterval(retry); window.removeEventListener('focus', acknowledgeVisible); document.removeEventListener('visibilitychange', acknowledgeVisible) }
  }, [room, enabled, messages])
}
