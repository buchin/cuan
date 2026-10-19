import { useEffect, useState } from 'react'
import { LoaderCircle, Square } from 'lucide-react'
import { InboxRequest } from '../../api/chat'

export function InboxActivity({
  request: q,
  onCancel,
}: {
  request: InboxRequest
  onCancel: () => Promise<void>
}) {
  const [now, setNow] = useState(Date.now())
  const [error, setError] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const [copyStatus, setCopyStatus] = useState('')
  const active =
    q.state === 'queued' || q.state === 'running' || q.state === 'cancelling'
  useEffect(() => {
    if (!active) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [active])
  const elapsed = Math.max(
    0,
    Math.floor((now - Date.parse(q.started_at || q.created_at)) / 1000),
  )
  const label = {
    queued: 'Pesan diterima · menunggu giliran',
    running: 'Agen sedang menyiapkan balasan',
    cancelling: 'Menghentikan proses…',
    completed: q.agent_id ? 'Balasan tersedia' : 'Pesan terkirim',
    failed: 'Agen belum dapat menjawab',
    interrupted: 'Proses terputus · hasil belum pasti',
    cancelled: 'Dibatalkan',
  }[q.state]
  return (
    <div className="mt-2 max-w-full rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs break-words">
      <div className="flex flex-wrap items-center gap-2">
        {active && (
          <LoaderCircle
            className="h-3 w-3 animate-spin shrink-0"
            aria-hidden="true"
          />
        )}
        <span className="font-medium" role="status">
          {label}
        </span>
        {active && (
          <span className="text-muted-foreground tabular-nums">
            {elapsed} dtk
          </span>
        )}
        {(q.state === 'queued' || q.state === 'running') && (
          <button
            type="button"
            disabled={cancelling}
            className="ml-auto inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
            onClick={async () => {
              setCancelling(true)
              setError('')
              try {
                await onCancel()
              } catch {
                setError(
                  'Pembatalan belum terkonfirmasi. Periksa status sebelum mencoba lagi.',
                )
              } finally {
                setCancelling(false)
              }
            }}
          >
            <Square className="h-3 w-3" />
            Batalkan
          </button>
        )}
      </div>
      {q.state === 'running' && q.steps?.length ? (
        <p className="mt-1 text-muted-foreground">
          {q.steps[q.steps.length - 1].text}
        </p>
      ) : null}
      {q.safe_error && (
        <p className="mt-1 text-muted-foreground">{q.safe_error}</p>
      )}
      {error && (
        <p role="alert" className="mt-1 text-destructive">
          {error}
        </p>
      )}
      <details className="mt-1 text-muted-foreground">
        <summary className="min-h-11 cursor-pointer py-3">
          Detail respons
        </summary>
        <ol className="mt-2 space-y-1">
          {q.steps?.map((step) => (
            <li key={step.seq}>
              <span>
                {step.seq}. {step.text}
              </span>
              {Number.isFinite(Date.parse(step.at)) && (
                <time
                  dateTime={step.at}
                  className="ml-2 inline-block text-xs tabular-nums"
                >
                  {new Date(step.at).toLocaleTimeString()}
                </time>
              )}
            </li>
          ))}
        </ol>
        <dl className="mt-3 space-y-2 text-xs">
          <div>
            <dt className="font-medium">ID permintaan</dt>
            <dd className="break-all font-mono">{q.request_id}</dd>
          </div>
          <div>
            <dt className="font-medium">ID pesan</dt>
            <dd className="break-all font-mono">{q.message_id}</dd>
          </div>
          <div>
            <dt className="font-medium">Status tersimpan</dt>
            <dd>{q.state}</dd>
          </div>
          {q.agent_id && (
            <div>
              <dt className="font-medium">Agen</dt>
              <dd className="break-all">{q.agent_id}</dd>
            </div>
          )}
          <div>
            <dt className="font-medium">Diterima</dt>
            <dd>{new Date(q.created_at).toLocaleString()}</dd>
          </div>
          {q.started_at && (
            <div>
              <dt className="font-medium">Mulai diproses</dt>
              <dd>{new Date(q.started_at).toLocaleString()}</dd>
            </div>
          )}
        </dl>
        {!active && <p className="mt-2">Status akhir ini sudah tersimpan.</p>}
        {!q.steps?.some((step) => step.kind === 'provider_result') &&
          !active && (
            <p className="mt-2">
              Metadata proses provider belum tersedia untuk permintaan ini.
            </p>
          )}
        <button
          type="button"
          className="my-2 min-h-11 underline"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(
                JSON.stringify(
                  {
                    request_id: q.request_id,
                    message_id: q.message_id,
                    agent_id: q.agent_id,
                    state: q.state,
                    created_at: q.created_at,
                    started_at: q.started_at,
                    steps: q.steps,
                    safe_error: q.safe_error,
                  },
                  null,
                  2,
                ),
              )
              setCopyStatus('Detail tersalin.')
            } catch {
              setCopyStatus(
                'Belum dapat menyalin. Pilih ID permintaan di atas untuk menyalin manual.',
              )
            }
          }}
        >
          Salin detail respons
        </button>
        {copyStatus && <p role="status">{copyStatus}</p>}
        <p>
          Pesan tersimpan saat halaman ditutup. Refresh tidak menjalankan ulang
          permintaan ini.
        </p>
      </details>
    </div>
  )
}
