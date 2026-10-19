import { useCallback, useEffect, useRef, useState } from 'react'
import { Bot, Plus, RefreshCw, Trash2, X } from 'lucide-react'
import { connectionStatus } from '../agents/connection'
import { accountsApi } from '../../api/accounts'
import { agentsApi, type Agent, type HarnessAuth } from '../../api/agents'

type Step = 'name' | 'google' | 'save'

export function TeamView({
  selectedRole = 'all',
  onAgentAdded,
  focusAgentId,
}: {
  focusAgentId?: string
  selectedRole?: string
  onAgentAdded?: () => void
}) {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState<Step>('name')
  const [handle, setHandle] = useState('@dev')
  const [auth, setAuth] = useState<HarnessAuth | null>(null)
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [checking, setChecking] = useState('')
  const [now, setNow] = useState(Date.now())
  // Expire visible success even when this roster stays open or resumes from sleep.
  useEffect(() => {
    const update = () => setNow(Date.now())
    const timer = window.setInterval(update, 1000)
    window.addEventListener('focus', update)
    document.addEventListener('visibilitychange', update)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('focus', update)
      document.removeEventListener('visibilitychange', update)
    }
  }, [])
  const active = useRef<{ accountId: string; sessionId: string } | null>(null)
  const accountScope = useRef('')
  const epoch = useRef(0)
  const dialogActive = useRef(false)
  const addButton = useRef<HTMLButtonElement>(null)

  const cancelSession = async () => {
    const session = active.current
    active.current = null
    if (session)
      await accountsApi
        .cancel(session.accountId, session.sessionId)
        .catch(() => {})
  }
  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setAgents(await agentsApi.list())
    } catch {
      setError('Daftar agen belum bisa dibuka. Silakan coba lagi.')
    } finally {
      setLoading(false)
    }
  }, [])
  useEffect(() => {
    void load()
  }, [load])
  useEffect(
    () => () => {
      dialogActive.current = false
      epoch.current++
      void cancelSession()
    },
    [],
  )
  useEffect(() => {
    if (!auth?.session_id || !auth.expires_at) return
    const sessionId = auth.session_id
    const timer = window.setTimeout(
      () => {
        if (active.current?.sessionId !== sessionId) return
        void cancelSession()
        setAuth(null)
        setCode('')
        setFormError('Waktu login habis. Buat link baru untuk melanjutkan.')
      },
      Math.max(0, Date.parse(auth.expires_at) - Date.now()),
    )
    return () => window.clearTimeout(timer)
  }, [auth?.session_id, auth?.expires_at])

  const close = () => {
    epoch.current++
    dialogActive.current = false
    void cancelSession()
    setOpen(false)
    setCode('')
    setAuth(null)
    setBusy(false)
    window.setTimeout(() => addButton.current?.focus(), 0)
  }
  const begin = () => {
    epoch.current++
    dialogActive.current = true
    accountScope.current = ''
    setStep('name')
    setOpen(true)
    setAuth(null)
    setCode('')
    setFormError('')
    setNotice('')
    setBusy(false)
    let suffix = 1
    let next = '@dev'
    while (
      agents.some(
        (a) =>
          a.name.split(' ')[0].toLowerCase() === next ||
          a.id === 'agent_' + next.slice(1),
      )
    )
      next = `@dev${++suffix}`
    setHandle(next)
  }
  const connect = async () => {
    if (busy || handle.length < 2) return
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    setCode('')
    try {
      await cancelSession()
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      if (!accountScope.current) {
        const account = await accountsApi.create(`${handle} · Google`)
        if (!dialogActive.current || epoch.current !== currentEpoch) return
        accountScope.current = account.id
      }
      const accountId = accountScope.current
      const session = await accountsApi.login(accountId)
      if (!dialogActive.current || epoch.current !== currentEpoch) {
        if (session.session_id)
          await accountsApi
            .cancel(accountId, session.session_id)
            .catch(() => {})
        return
      }
      if (!session.auth_url || !session.session_id)
        throw new Error('Link Google belum tersedia. Silakan coba lagi.')
      active.current = { accountId, sessionId: session.session_id }
      setAuth(session)
      setStep('google')
    } catch (err) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          err instanceof Error
            ? err.message
            : 'Google belum bisa dihubungkan. Silakan coba lagi.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) setBusy(false)
    }
  }
  const finish = async (currentEpoch: number) => {
    await agentsApi.connectCLI({
      name: handle,
      handle,
      provider: 'agy',
      account_id: accountScope.current,
    })
    if (!dialogActive.current || epoch.current !== currentEpoch) return
    close()
    setNotice(`${handle} berhasil ditambahkan.`)
    await load()
    onAgentAdded?.()
  }
  const retrySave = async () => {
    const currentEpoch = epoch.current
    setBusy(true)
    setFormError('')
    try {
      await finish(currentEpoch)
    } catch (e) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          e instanceof Error
            ? e.message
            : 'Agen belum bisa disimpan. Coba lagi.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) setBusy(false)
    }
  }
  const check = async (id: string) => {
    setChecking(id)
    setError('')
    setNotice('')
    try {
      const result = await agentsApi.verifyConnection(id)
      setAgents((current) =>
        current.map((a) =>
          a.id === id
            ? {
                ...a,
                connection_state: result.connection_state,
                last_checked_at: result.last_checked_at,
                verified_at: result.verified_at,
              }
            : a,
        ),
      )
      const messages: Record<string, string> = {
        verified: 'Provider berhasil merespons. Hasil berlaku lima menit.',
        quota_limited:
          'Kuota agen sedang habis. Coba lagi setelah kuota pulih.',
        login_required: 'Agen perlu login kembali.',
        unavailable: 'Koneksi sedang terganggu. Coba lagi nanti.',
      }
      setNotice(
        messages[result.connection_state] ||
          'Koneksi belum dapat dipastikan. Coba lagi nanti.',
      )
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Koneksi belum dapat diperiksa. Coba lagi.',
      )
    } finally {
      setChecking('')
    }
  }
  const verify = async () => {
    if (!active.current || !code.trim() || busy) return
    const currentEpoch = epoch.current
    const session = active.current
    setBusy(true)
    setFormError('')
    try {
      const account = await accountsApi.code(
        session.accountId,
        session.sessionId,
        code.trim(),
      )
      if (!dialogActive.current || epoch.current !== currentEpoch) return
      if (!account.credentials_present)
        throw new Error(
          'Login belum selesai. Buka Google dan salin kode yang diberikan.',
        )
      active.current = null
      setAuth(null)
      setStep('save')
      await finish(currentEpoch)
    } catch (err) {
      if (dialogActive.current && epoch.current === currentEpoch)
        setFormError(
          err instanceof Error
            ? err.message
            : 'Kode belum berhasil diperiksa. Coba lagi atau buat link baru.',
        )
    } finally {
      if (dialogActive.current && epoch.current === currentEpoch) {
        setCode('')
        setBusy(false)
      }
    }
  }
  const remove = async (id: string) => {
    setError('')
    try {
      await agentsApi.delete(id)
      setAgents((current) => current.filter((a) => a.id !== id))
      onAgentAdded?.()
    } catch {
      setError('Agen belum bisa dihapus. Silakan coba lagi.')
    }
  }
  useEffect(() => {
    if (focusAgentId && !loading)
      document
        .getElementById(`settings-agent-${focusAgentId}`)
        ?.scrollIntoView({ block: 'center' })
  }, [focusAgentId, loading])
  const visibleAgents =
    selectedRole === 'human' || selectedRole === 'product_owner' ? [] : agents
  const secondary =
    'inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-2.5 text-sm hover:bg-muted disabled:opacity-50'
  const primary =
    'inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50'

  return (
    <div className="mx-auto max-w-4xl space-y-5 pb-8">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Agen</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Satu agen, satu koneksi.
          </p>
        </div>
        <button
          ref={addButton}
          onClick={() => begin()}
          className={`${primary} shrink-0`}
        >
          <Plus className="h-4 w-4" />
          Tambah agen
        </button>
      </div>
      {notice && (
        <p role="status" className="text-sm">
          {notice}
        </p>
      )}
      {error && (
        <div role="alert" className="text-sm text-destructive">
          {error}{' '}
          <button onClick={load} className="underline">
            Coba lagi
          </button>
        </div>
      )}
      {loading ? (
        <p role="status" className="text-sm text-muted-foreground">
          Memuat agen…
        </p>
      ) : visibleAgents.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-8 text-center">
          <Bot className="mx-auto h-8 w-8 text-muted-foreground" />
          <h2 className="mt-3 font-medium">Belum ada agen</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Tambahkan agen pertama Anda, lalu hubungkan akun Google.
          </p>
          <button onClick={() => begin()} className={`${primary} mt-4`}>
            Tambah agen pertama
          </button>
        </div>
      ) : (
        <div
          role="list"
          aria-label="Daftar agen"
          className="divide-y divide-border overflow-hidden rounded-lg border border-border"
        >
          {visibleAgents.map((agent) => {
            const expiry = agent.verified_at
              ? Date.parse(agent.verified_at) + 5 * 60_000
              : NaN
            const state = connectionStatus(agent, now)
            const a = { ...agent, connection_state: state.state }
            const details: Record<string, string> = {
              verified: 'Provider berhasil merespons.',
              unknown:
                'Belum ada hasil yang masih berlaku. Periksa koneksi tanpa login ulang.',
              login_required: 'Login diperlukan; riwayat tetap tersimpan.',
              quota_limited:
                'Provider membatasi kuota. Tunggu kuota pulih; login ulang tidak diperlukan.',
              unavailable:
                'Provider belum dapat dijangkau. Coba periksa lagi nanti.',
            }
            return (
              <div
                role="listitem"
                aria-label={`Agen ${a.name}`}
                key={a.id}
                id={`settings-agent-${a.id}`}
                className="p-4"
              >
                <div className="flex items-start gap-3">
                  <Bot className="mt-2 h-5 w-5 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-base font-medium">{a.name}</h3>
                    <p
                      className={`mt-1 text-sm ${state.verified ? 'text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground'}`}
                    >
                      {state.label}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    {(a.harness === 'agy' ||
                      a.provider === 'agy' ||
                      a.provider === 'antigravity') && (
                      <button
                        onClick={() => void check(a.id)}
                        disabled={!!checking}
                        aria-label={`Periksa ulang koneksi ${a.name}`}
                        title={`Periksa ulang koneksi ${a.name}`}
                        className="icon-action border border-border disabled:opacity-50"
                      >
                        <RefreshCw
                          className={`h-4 w-4 ${checking === a.id ? 'animate-spin' : ''}`}
                        />
                      </button>
                    )}
                    <button
                      onClick={() => void remove(a.id)}
                      aria-label={`Hapus ${a.name}`}
                      className="icon-action text-muted-foreground hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>
                <p className="mt-3 text-sm text-muted-foreground">
                  {state.state === 'disabled'
                    ? 'Agen ini tidak aktif.'
                    : details[a.connection_state || 'unknown'] ||
                      details.unknown}
                </p>
                {(a.last_checked_at || a.verified_at) && (
                  <details className="mt-2 text-xs text-muted-foreground">
                    <summary className="inline-flex min-h-11 cursor-pointer items-center text-sm underline underline-offset-2">
                      Detail pemeriksaan
                    </summary>
                    <div className="space-y-1 pb-2">
                      {a.last_checked_at && (
                        <p>
                          Diperiksa:{' '}
                          <time dateTime={a.last_checked_at}>
                            {new Date(a.last_checked_at).toLocaleString()}
                          </time>
                        </p>
                      )}
                      {a.verified_at && Number.isFinite(expiry) && (
                        <p>
                          {state.verified
                            ? 'Berlaku sampai'
                            : 'Hasil pemeriksaan kedaluwarsa pada'}
                          :{' '}
                          <time dateTime={new Date(expiry).toISOString()}>
                            {new Date(expiry).toLocaleString()}
                          </time>
                        </p>
                      )}
                    </div>
                  </details>
                )}
              </div>
            )
          })}
        </div>
      )}
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm">
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="add-agent-title"
            className="max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-xl border border-border bg-card shadow-xl"
            onKeyDown={(event) => {
              if (event.key === 'Escape') close()
              if (event.key !== 'Tab') return
              const items = Array.from(
                event.currentTarget.querySelectorAll<HTMLElement>(
                  'button:not(:disabled), input:not(:disabled), a[href]',
                ),
              )
              const first = items[0],
                last = items[items.length - 1]
              if (event.shiftKey && document.activeElement === first) {
                event.preventDefault()
                last?.focus()
              } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault()
                first?.focus()
              }
            }}
          >
            <header className="flex items-center justify-between border-b border-border px-5 py-4">
              <div>
                <h2 id="add-agent-title" className="font-semibold">
                  Tambah agen
                </h2>
                {step !== 'save' && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {step === 'name'
                      ? 'Langkah 1 dari 2 · Beri nama'
                      : 'Langkah 2 dari 2 · Hubungkan Google'}
                  </p>
                )}
              </div>
              <button
                onClick={close}
                aria-label="Tutup"
                className="icon-action rounded p-2 hover:bg-muted"
              >
                <X className="h-4 w-4" />
              </button>
            </header>
            <form
              onSubmit={(event) => {
                event.preventDefault()
                if (step === 'save') void retrySave()
                else if (step === 'name' || !auth?.auth_url) void connect()
                else void verify()
              }}
            >
              <div className="space-y-4 p-5">
                {step === 'name' && (
                  <>
                    <div>
                      <label
                        htmlFor="agent-handle"
                        className="text-sm font-medium"
                      >
                        Nama agen
                      </label>
                      <input
                        id="agent-handle"
                        autoFocus
                        required
                        maxLength={49}
                        value={handle}
                        onChange={(event) =>
                          setHandle(
                            '@' +
                              event.target.value
                                .replace(/^@+/, '')
                                .toLowerCase()
                                .replace(/[^a-z0-9_-]/g, ''),
                          )
                        }
                        className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                        aria-describedby="agent-name-help"
                      />
                      <p
                        id="agent-name-help"
                        className="mt-2 text-xs text-muted-foreground"
                      >
                        Contoh: @penulis atau @developer.
                      </p>
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Selanjutnya, masuk ke Google untuk menghubungkan akun agen
                      ini. Koneksi agen lain tetap tersimpan.
                    </p>
                  </>
                )}
                {step === 'google' && (
                  <>
                    <p className="text-sm">
                      Hubungkan Google untuk <strong>{handle}</strong>.
                    </p>
                    {auth?.auth_url ? (
                      <>
                        <div className="space-y-2">
                          <p className="text-sm font-medium">
                            1. Masuk ke akun Google Anda
                          </p>
                          <a
                            href={auth.auth_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={`${secondary} w-full`}
                          >
                            Buka Google ↗
                          </a>
                          <p className="text-xs text-muted-foreground">
                            Pilih akun yang ingin digunakan. Setelah selesai,
                            kembali ke sini.
                          </p>
                        </div>
                        <div>
                          <label
                            htmlFor="oauth-code"
                            className="text-sm font-medium"
                          >
                            2. Tempel kode dari Google
                          </label>
                          <input
                            id="oauth-code"
                            type="password"
                            autoComplete="off"
                            value={code}
                            onChange={(event) => setCode(event.target.value)}
                            placeholder="Tempel kode di sini"
                            className="mt-2 w-full rounded-md border border-input bg-background px-3 py-2.5 text-base"
                          />
                          <p className="mt-2 text-xs text-muted-foreground">
                            Kode hanya dimasukkan di sini. Link berlaku 5 menit.
                          </p>
                        </div>
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => void connect()}
                          className="text-xs text-muted-foreground underline"
                        >
                          Buat link baru
                        </button>
                      </>
                    ) : (
                      <p className="text-sm text-muted-foreground">
                        Buat link baru untuk melanjutkan login Google.
                      </p>
                    )}
                  </>
                )}
                {step === 'save' && (
                  <p className="text-sm">
                    Google sudah terhubung. Simpan <strong>{handle}</strong>{' '}
                    untuk mulai menggunakannya.
                  </p>
                )}
                {formError && (
                  <p
                    role="alert"
                    className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
                  >
                    {formError}
                  </p>
                )}
                {busy && (
                  <p role="status" className="text-sm text-muted-foreground">
                    {step === 'save'
                      ? 'Menambahkan agen…'
                      : step === 'name' || !auth?.auth_url
                        ? 'Menyiapkan login Google…'
                        : 'Memeriksa kode…'}
                  </p>
                )}
              </div>
              <footer className="flex items-center justify-end gap-2 border-t border-border p-4">
                {step !== 'save' && (
                  <button type="button" onClick={close} className={secondary}>
                    Batal
                  </button>
                )}
                <button
                  type="submit"
                  className={primary}
                  disabled={
                    busy ||
                    (step === 'name' && handle.length < 2) ||
                    (step === 'google' && !!auth?.auth_url && !code.trim())
                  }
                >
                  {busy && <RefreshCw className="h-4 w-4 animate-spin" />}
                  {step === 'save'
                    ? 'Simpan agen'
                    : step === 'name'
                      ? 'Hubungkan Google'
                      : auth?.auth_url
                        ? 'Konfirmasi login'
                        : 'Buat link baru'}
                </button>
              </footer>
            </form>
          </section>
        </div>
      )}
    </div>
  )
}

export function TeamSubPanel({
  onAddAgentClick,
}: {
  selectedRole: string
  onSelectRole: (role: string) => void
  onAddAgentClick?: () => void
}) {
  return (
    <div className="space-y-3 p-4">
      <h2 className="text-sm font-medium">Agen</h2>
      <p className="text-xs text-muted-foreground">Satu agen, satu koneksi.</p>
      {onAddAgentClick && (
        <button
          onClick={onAddAgentClick}
          className="flex items-center gap-2 text-sm text-primary"
        >
          <Plus className="h-4 w-4" />
          Tambah agen
        </button>
      )}
    </div>
  )
}
