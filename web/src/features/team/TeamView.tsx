import { useCallback, useEffect, useRef, useState } from 'react'
import { Bot, Plus, RefreshCw, Trash2, X } from 'lucide-react'
import { ProviderAccounts } from './ProviderAccounts'
import { accountsApi } from '../../api/accounts'
import { agentsApi, type Agent, type HarnessAuth } from '../../api/agents'

export function TeamView({ selectedRole = 'all', onAgentAdded }: {
  selectedRole?: string
  onAgentAdded?: () => void
}) {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [open, setOpen] = useState(false)
  const [handle, setHandle] = useState('@dev')
  const [auth, setAuth] = useState<HarnessAuth | null>(null)
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [formError, setFormError] = useState('')
  const dialogActive = useRef(false)
  const activeSession = useRef('')
  const accountScope = useRef('')
  const dialogEpoch = useRef(0)
  const [newAccount, setNewAccount] = useState(false)
  const cancelSession = async () => {
    const session = activeSession.current
    activeSession.current = ''
    if (!session) return
    if (accountScope.current) await accountsApi.cancel(accountScope.current, session).catch(() => {})
    else await agentsApi.authCancel(session).catch(() => {})
  }

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try { setAgents(await agentsApi.list()) }
    catch { setError('Daftar agen gagal dimuat. Coba lagi.') }
    finally { setLoading(false) }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => () => {
    dialogActive.current = false
    void cancelSession()
  }, [])

  useEffect(() => {
    if (!auth?.session_id || !auth.expires_at) return
    const session = auth.session_id
    const timer = window.setTimeout(() => {
      if (activeSession.current !== session) return
      void cancelSession()
      activeSession.current = ''
      setAuth(null)
      setCode('')
      setFormError('Sesi login kedaluwarsa. Klik Login dengan Google untuk mulai lagi.')
    }, Math.max(0, Date.parse(auth.expires_at) - Date.now()))
    return () => window.clearTimeout(timer)
  }, [auth?.session_id, auth?.expires_at])

  const close = () => {
    dialogEpoch.current++
    dialogActive.current = false
    void cancelSession()
    activeSession.current = ''
    setOpen(false)
    setCode('')
    setAuth(null)
    setBusy(false)
  }

  const begin = () => {
    dialogEpoch.current++
    dialogActive.current = true
    accountScope.current = ''
    setNewAccount(true)
    setOpen(true)
    setAuth(null)
    setFormError('')
    setNotice('')
    setBusy(false)
    let suffix = 1
    let next = '@dev'
    while (agents.some(a => a.name.split(' ')[0].toLowerCase() === next || a.id === 'agent_' + next.slice(1))) next = `@dev${++suffix}`
    setHandle(next)
  }

  const chooseNewAccount = async () => {
    const epoch = dialogEpoch.current
    setBusy(true)
    setFormError('')
    setCode('')
    try {
      await cancelSession()
      if (!dialogActive.current || dialogEpoch.current !== epoch) return
      setNewAccount(true)
      if (!accountScope.current) {
        const account = await accountsApi.create(`${handle || '@dev'} · Google`)
        if (!dialogActive.current || dialogEpoch.current !== epoch) return
        accountScope.current = account.id
      }
      const id = accountScope.current
      const session = await accountsApi.login(id)
      if (!dialogActive.current || dialogEpoch.current !== epoch) {
        if (session.session_id) await accountsApi.cancel(id, session.session_id).catch(() => {})
        return
      }
      activeSession.current = session.session_id || ''
      setAuth(session)
    } catch (err) { if (dialogActive.current && dialogEpoch.current === epoch) setFormError(err instanceof Error ? err.message : 'Login gagal. Coba lagi.') }
    finally { if (dialogActive.current && dialogEpoch.current === epoch) setBusy(false) }
  }


  const verifyCode = async () => {
    const epoch = dialogEpoch.current
    if (!activeSession.current || !code.trim()) return
    setBusy(true)
    setFormError('')
    try {
      const account = accountScope.current ? await accountsApi.code(accountScope.current, activeSession.current, code.trim()) : null
      const status = account ? { authenticated: account.state === 'verified', verified: account.state === 'verified', credentials_present: account.credentials_present, message: account.message } : await agentsApi.authCode(activeSession.current, code.trim())
      if (dialogActive.current && dialogEpoch.current === epoch) { setAuth(status); activeSession.current = '' }
    } catch (err) { if (dialogActive.current && dialogEpoch.current === epoch) setFormError(err instanceof Error ? err.message : 'Kode belum berhasil diverifikasi. Mulai ulang login.') }
    finally { if (dialogActive.current && dialogEpoch.current === epoch) { setCode(''); setBusy(false) } }
  }

  const add = async (event: React.FormEvent) => {
    event.preventDefault()
    if (newAccount || !auth?.verified || handle.length < 2) return
    setBusy(true)
    setFormError('')
    try {
      await agentsApi.connectCLI({ name: handle, handle, provider: 'agy', auth_type: 'oauth' })
      close()
      setHandle('@dev')
      setNotice('Agen ditambahkan setelah koneksi provider berhasil diverifikasi.')
      await load()
      onAgentAdded?.()
    } catch (err) { setFormError(err instanceof Error ? err.message : 'Agen gagal ditambahkan. Coba lagi.') }
    finally { setBusy(false) }
  }

  const remove = async (id: string) => {
    setError('')
    try {
      await agentsApi.delete(id)
      setAgents(current => current.filter(agent => agent.id !== id))
      onAgentAdded?.()
    } catch { setError('Agen gagal dihapus. Coba lagi.') }
  }

  const visibleAgents = selectedRole === 'human' || selectedRole === 'product_owner' ? [] : agents
  const buttonClass = 'inline-flex items-center justify-center gap-2 rounded-md border border-border px-3 py-2 text-sm disabled:opacity-50 hover:bg-muted'

  return <div className="max-w-4xl mx-auto space-y-5 pb-8">
    <div className="flex items-start justify-between gap-4">
      <div>
        <h1 className="text-xl font-semibold">Agen</h1>
        <p className="mt-1 text-sm text-muted-foreground">Kelola agen dan akun yang terhubung.</p>
      </div>
      <button onClick={begin} className={`${buttonClass} bg-primary text-primary-foreground hover:bg-primary/90 shrink-0`}><Plus className="h-4 w-4" />Tambah agen</button>
    </div>
    <details className="rounded-lg border border-border p-3"><summary className="cursor-pointer text-sm">Kelola akun Google</summary><div className="mt-3"><ProviderAccounts /></div></details>
    {notice && <p role="status" className="text-sm">{notice}</p>}
    {error && <div role="alert" className="text-sm text-destructive">{error} <button onClick={load} className="underline">Muat ulang</button></div>}
    {loading ? <p role="status" className="text-sm text-muted-foreground">Memuat agen…</p> : visibleAgents.length === 0 ?
      <div className="rounded-lg border border-dashed border-border p-10 text-center">
        <Bot className="mx-auto h-8 w-8 text-muted-foreground" />
        <h2 className="mt-3 font-medium">Belum ada agen</h2>
        <p className="mt-1 text-sm text-muted-foreground">Tambahkan agen dengan akun Anda sendiri. Login akan diverifikasi sebelum agen disimpan.</p>
        <button onClick={begin} className={`${buttonClass} mt-4`}>Tambah agen pertama</button>
      </div> : <div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
        {visibleAgents.map(agent => <div key={agent.id} className="flex items-center gap-3 p-4">
          <Bot className="h-5 w-5 shrink-0 text-muted-foreground" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{agent.name}</p>
            <p className="text-xs text-muted-foreground">{agent.harness || agent.provider || 'Provider belum dipilih'} · {agent.enabled === false || agent.status === 'inactive' ? 'Dinonaktifkan' : 'Terdaftar'}</p>
          </div>
          <button onClick={() => remove(agent.id)} aria-label={`Hapus ${agent.name}`} className="rounded p-2 text-muted-foreground hover:bg-muted hover:text-destructive"><Trash2 className="h-4 w-4" /></button>
        </div>)}
      </div>}
    {open && <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm">
      <section role="dialog" aria-modal="true" aria-labelledby="add-agent-title" onKeyDown={event => { if (event.key === 'Escape') close() }} className="max-h-[90vh] w-full max-w-md overflow-y-auto rounded-xl border border-border bg-card shadow-xl">
        <header className="flex items-center justify-between border-b border-border p-5">
          <h2 id="add-agent-title" className="font-semibold">Tambah agen</h2>
          <button onClick={close} aria-label="Tutup" className="rounded p-1 hover:bg-muted"><X className="h-4 w-4" /></button>
        </header>
        <form onSubmit={add} className="space-y-4 p-5">
          <div>
            <label htmlFor="agent-handle" className="text-sm font-medium">Nama agen</label>
            <input id="agent-handle" autoFocus required maxLength={49} value={handle} onChange={event => setHandle('@' + event.target.value.replace(/^@+/, '').toLowerCase().replace(/[^a-z0-9_-]/g, ''))} className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" placeholder="@dev" />
          </div>
          <div className="rounded-md border border-border p-3 text-sm space-y-3">
            <p className="font-medium">Akun Google</p>
            <p className="text-xs text-muted-foreground">Hubungkan akun Google untuk agen ini.</p>
            <button type="button" onClick={() => void chooseNewAccount()} disabled={busy || handle.length < 2 || !!auth?.credentials_present} className={buttonClass}>{auth?.session_id ? 'Mulai ulang login' : 'Login dengan Google'}</button>
            <p role="status" className="text-muted-foreground">{busy ? 'Memproses…' : auth?.verified ? newAccount ? 'Login akun baru tersimpan.' : 'Akun lama terverifikasi.' : auth?.message || 'Hubungkan akun Anda.'}</p>

            {auth?.auth_url && !auth.verified && <div className="space-y-3">
              <a href={auth.auth_url} target="_blank" rel="noopener noreferrer" className="block text-primary underline">Buka login Google ↗</a>
              <p className="text-xs text-muted-foreground">Selesaikan login di browser, lalu tempel kode otorisasi. Sesi berlaku 5 menit.</p>
              <label htmlFor="oauth-code" className="block text-sm">Kode otorisasi</label>
              <input id="oauth-code" type="password" autoComplete="off" value={code} onChange={event => setCode(event.target.value)} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm" />
              <button type="button" onClick={verifyCode} disabled={busy || !code.trim()} className={buttonClass}>Verifikasi login</button>
            </div>}
          </div>
          {newAccount && auth?.credentials_present && <p className="text-sm">Akun baru tersimpan. Penggunaan akun baru untuk agen menunggu verifikasi tahap berikutnya.</p>}
          {formError && <p role="alert" className="text-sm text-destructive">{formError}</p>}
          <div className="flex justify-end gap-2 border-t border-border pt-4">
            <button type="button" onClick={close} className={buttonClass}>Batal</button>
            <button type="submit" disabled={busy || newAccount || !auth?.verified || handle.length < 2} className={`${buttonClass} bg-primary text-primary-foreground hover:bg-primary/90`}>{busy && <RefreshCw className="h-4 w-4 animate-spin" />}Tambah agen</button>
          </div>
        </form>
      </section>
    </div>}
  </div>
}

export function TeamSubPanel({ onAddAgentClick }: {
  selectedRole: string
  onSelectRole: (role: string) => void
  onAddAgentClick?: () => void
}) {
  return <div className="p-4 space-y-3">
    <h2 className="text-sm font-medium">Agen</h2>
    <p className="text-xs text-muted-foreground">Kelola agen yang Anda hubungkan sendiri.</p>
    {onAddAgentClick && <button onClick={onAddAgentClick} className="flex items-center gap-2 text-sm text-primary"><Plus className="h-4 w-4" />Tambah agen</button>}
  </div>
}
