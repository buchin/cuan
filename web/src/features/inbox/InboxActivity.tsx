import { useEffect, useState } from 'react'
import {
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Clock3,
  LoaderCircle,
  Square,
} from 'lucide-react'
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
  const active = ['queued', 'running', 'cancelling'].includes(q.state)
  useEffect(() => {
    if (!active) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [active])
  const start = Date.parse(q.started_at || q.created_at)
  const finished = q.steps?.find((step) => step.kind === 'provider_result')
  const end = finished ? Date.parse(finished.at) : NaN
  const duration = active ? now - start : end - start
  const seconds =
    Number.isFinite(duration) && duration >= 0
      ? Math.floor(duration / 1000)
      : null
  const label = {
    queued: 'Menunggu',
    running: 'Memproses',
    cancelling: 'Menghentikan…',
    completed: 'Selesai',
    failed: 'Gagal',
    interrupted: 'Terputus',
    cancelled: 'Dibatalkan',
  }[q.state]
  const processSteps =
    q.steps?.filter((step) => step.kind !== 'provider_result') || []
  const Icon =
    q.state === 'queued'
      ? Clock3
      : active
        ? LoaderCircle
        : q.state === 'completed'
          ? CheckCircle2
          : CircleAlert
  return (
    <div
      className="mt-2 max-w-full min-w-0 text-xs break-words"
      aria-label="Status respons"
    >
      <div className="flex items-start gap-2">
        <details className="group/response min-w-0 flex-1">
          <summary className="flex min-h-11 cursor-pointer list-none items-center gap-2 rounded-md px-2 text-muted-foreground hover:bg-muted/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
            <Icon
              className={`h-4 w-4 shrink-0 ${active && q.state !== 'queued' ? 'animate-spin' : q.state === 'failed' ? 'text-destructive' : q.state === 'completed' ? 'text-emerald-700 dark:text-emerald-400' : ''}`}
              aria-hidden="true"
            />
            <span className="font-medium" role="status">
              {label}
            </span>
            {seconds !== null && (
              <span className="tabular-nums">
                {seconds} dtk{!active && ' tercatat'}
              </span>
            )}
            <span className="ml-auto text-xs">Detail respons</span>
            <ChevronDown
              className="h-4 w-4 shrink-0 transition-transform group-open/response:rotate-180"
              aria-hidden="true"
            />
          </summary>
          <div className="mt-1 space-y-3 rounded-lg border border-border/60 bg-muted/20 p-3 text-sm">
            <p className="font-medium">
              {q.state === 'completed'
                ? q.agent_id
                  ? 'Balasan tersimpan.'
                  : 'Pesan terkirim.'
                : q.state === 'queued'
                  ? 'Pesan tersimpan dan menunggu giliran.'
                  : q.state === 'running'
                    ? 'Agen sedang menyiapkan balasan.'
                    : q.state === 'interrupted'
                      ? 'Proses terputus. Hasil provider belum dapat dipastikan.'
                      : q.state === 'cancelled'
                        ? 'Permintaan dibatalkan. Pesan tetap tersimpan.'
                        : q.state === 'cancelling'
                          ? 'Menunggu konfirmasi penghentian proses.'
                          : 'Agen belum dapat menjawab. Pesan tetap tersimpan.'}
            </p>
            {q.safe_error && (
              <p className="text-muted-foreground">{q.safe_error}</p>
            )}
            {processSteps.length ? (
              <ol
                aria-label="Timeline respons"
                className="space-y-3 border-l border-border pl-3"
              >
                {processSteps.map((step) => (
                  <li key={step.seq} className="space-y-1">
                    <p>{step.text}</p>
                    {Number.isFinite(Date.parse(step.at)) && (
                      <time
                        dateTime={step.at}
                        className="block text-xs tabular-nums text-muted-foreground"
                      >
                        {new Date(step.at).toLocaleTimeString()}
                      </time>
                    )}
                  </li>
                ))}
              </ol>
            ) : (
              <p className="text-muted-foreground">
                Belum ada aktivitas yang direkam.
              </p>
            )}
            <details className="border-t border-border pt-1 text-muted-foreground">
              <summary className="min-h-11 cursor-pointer py-3">
                Detail teknis
              </summary>
              <dl className="space-y-3 text-xs">
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
                {finished && (
                  <div>
                    <dt className="font-medium">Hasil proses CLI</dt>
                    <dd>{finished.text}</dd>
                  </div>
                )}
              </dl>
              {!finished && !active && (
                <p className="mt-3 text-xs">
                  Metadata proses provider belum tersedia untuk permintaan ini.
                </p>
              )}
              <button
                type="button"
                className="my-2 min-h-11 text-sm underline"
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
                Salin detail teknis
              </button>
              {copyStatus && (
                <p role="status" className="text-xs">
                  {copyStatus}
                </p>
              )}
              <p className="text-xs">
                Membuka detail atau refresh tidak menjalankan ulang permintaan.
                Detail hanya memuat kejadian yang direkam; tidak menyertakan
                reasoning privat atau kredensial.
              </p>
            </details>
          </div>
        </details>
        {(q.state === 'queued' || q.state === 'running') && (
          <button
            type="button"
            disabled={cancelling}
            className="inline-flex min-h-11 shrink-0 items-center gap-1 px-2 text-muted-foreground underline disabled:opacity-50"
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
      {error && (
        <p role="alert" className="mt-1 text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
