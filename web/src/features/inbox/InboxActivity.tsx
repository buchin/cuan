import { useEffect, useState } from 'react'
import { LoaderCircle, Square } from 'lucide-react'
import { InboxRequest } from '../../api/chat'

export function InboxActivity({ request: q, onCancel }: { request: InboxRequest; onCancel: () => Promise<void> }) {
  const [now, setNow] = useState(Date.now())
  const [error, setError] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const active = q.state === 'queued' || q.state === 'running' || q.state === 'cancelling'
  useEffect(() => { if (!active) return; const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer) }, [active])
  const elapsed = Math.max(0, Math.floor((now - Date.parse(q.started_at || q.created_at)) / 1000))
  const label = { queued: 'Pesan diterima · menunggu giliran', running: 'Agen sedang menyiapkan balasan', cancelling: 'Menghentikan proses…', completed: q.agent_id ? 'Balasan tersedia' : 'Pesan terkirim', failed: 'Agen belum dapat menjawab', interrupted: 'Proses terputus · hasil belum pasti', cancelled: 'Dibatalkan' }[q.state]
  return <div className="mt-2 max-w-full rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs break-words">
    <div className="flex flex-wrap items-center gap-2">
      {active && <LoaderCircle className="h-3 w-3 animate-spin shrink-0" aria-hidden="true" />}
      <span className="font-medium" role="status">{label}</span>
      {active && <span className="text-muted-foreground tabular-nums">{elapsed} dtk</span>}
      {(q.state === 'queued' || q.state === 'running') && <button type="button" disabled={cancelling} className="ml-auto inline-flex items-center gap-1 underline disabled:opacity-50" onClick={async () => { setCancelling(true); setError(''); try { await onCancel() } catch { setError('Pembatalan belum terkonfirmasi. Periksa status sebelum mencoba lagi.') } finally { setCancelling(false) } }}><Square className="h-3 w-3" />Batalkan</button>}
    </div>
    {q.state === 'running' && q.steps?.length ? <p className="mt-1 text-muted-foreground">{q.steps[q.steps.length - 1].text}</p> : null}
    {q.safe_error && <p className="mt-1 text-muted-foreground">{q.safe_error}</p>}
    {error && <p role="alert" className="mt-1 text-destructive">{error}</p>}
    <details className="mt-1 text-muted-foreground"><summary className="cursor-pointer">Detail aktivitas</summary>
      <ol className="mt-2 space-y-1">{q.steps?.map(step => <li key={step.seq}>{step.seq}. {step.text}</li>)}</ol>
      <p className="mt-2">ID pesan: {q.message_id}</p>
      <p>Pesan tersimpan saat halaman ditutup. Refresh tidak menjalankan ulang permintaan ini.</p>
    </details>
  </div>
}
