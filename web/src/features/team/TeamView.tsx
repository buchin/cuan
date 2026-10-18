import { useCallback, useEffect, useRef, useState } from 'react'
import { Bot, Plus, RefreshCw, Trash2, X } from 'lucide-react'
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
    if (activeSession.current) void agentsApi.authCancel(activeSession.current).catch(() => {})
  }, [])

  const close = () => {
    dialogActive.current = false
    if (activeSession.current) void agentsApi.authCancel(activeSession.current).catch(() => {})
    activeSession.current = ''
    setOpen(false)
    setCode('')
    setAuth(null)
    setBusy(false)
  }

  const begin = async () => {
    dialogActive.current = true
    setOpen(true)
    setAuth(null)
    setFormError('')
    setNotice('')
    setBusy(true)
    try {
      const status = await agentsApi.authStatus('agy', true)
      if (dialogActive.current) setAuth(status)
    } catch { if (dialogActive.current) setFormError('Status koneksi gagal diperiksa. Coba lagi.') }
    finally { if (dialogActive.current) setBusy(false) }
  }

  const login = async () => {
    setFormError('')
    setBusy(true)
    setCode('')
    try {
      const session = await agentsApi.authInit()
      if (!dialogActive.current) {
        if (session.session_id) void agentsApi.authCancel(session.session_id).catch(() => {})
        return
      }
      activeSession.current = session.session_id || ''
      setAuth(session)
    } catch (err) { if (dialogActive.current) setFormError(err instanceof Error ? err.message : 'Login gagal. Coba lagi.') }
    finally { if (dialogActive.current) setBusy(false) }
  }

  const verifyCode = async () => {
    if (!activeSession.current || !code.trim()) return
    setBusy(true)
    setFormError('')
    try {
      const status = await agentsApi.authCode(activeSession.current, code.trim())
      if (dialogActive.current) { setAuth(status); activeSession.current = '' }
    } catch (err) { if (dialogActive.current) setFormError(err instanceof Error ? err.message : 'Kode belum berhasil diverifikasi. Mulai ulang login.') }
    finally { if (dialogActive.current) { setCode(''); setBusy(false) } }
  }

  const add = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!auth?.verified || handle.length < 2) return
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
        <p className="mt-1 text-sm text-muted-foreground">Hubungkan akun Anda untuk mengobrol dan menjalankan tugas.</p>
      </div>
      <button onClick={begin} className={`${buttonClass} bg-primary text-primary-foreground hover:bg-primary/90 shrink-0`}><Plus className="h-4 w-4" />Tambah agen</button>
    </div>
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
            <input id="agent-handle" autoFocus required value={handle} onChange={event => setHandle('@' + event.target.value.replace(/^@+/, '').toLowerCase().replace(/[^a-z0-9_-]/g, ''))} className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" placeholder="@dev" />
          </div>
          <div className="rounded-md border border-border p-3 text-sm space-y-3">
            <p className="font-medium">Antigravity · Akun Google</p>
            <p role="status" className="text-muted-foreground">{busy ? 'Memproses…' : auth?.verified ? 'Koneksi provider terverifikasi.' : auth?.message || 'Hubungkan akun Anda.'}</p>
            {!auth?.verified && <button type="button" onClick={login} disabled={busy} className={buttonClass}>{auth?.session_id ? 'Mulai ulang login' : 'Login dengan Google'}</button>}
            {auth?.auth_url && !auth.verified && <div className="space-y-3">
              <a href={auth.auth_url} target="_blank" rel="noopener noreferrer" className="block text-primary underline">Buka login Google ↗</a>
              <p className="text-xs text-muted-foreground">Selesaikan login di browser, lalu tempel kode otorisasi. Sesi berlaku 5 menit.</p>
              <label htmlFor="oauth-code" className="block text-sm">Kode otorisasi</label>
              <input id="oauth-code" type="password" autoComplete="off" value={code} onChange={event => setCode(event.target.value)} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm" />
              <button type="button" onClick={verifyCode} disabled={busy || !code.trim()} className={buttonClass}>Verifikasi login</button>
            </div>}
          </div>
          {formError && <p role="alert" className="text-sm text-destructive">{formError}</p>}
          <div className="flex justify-end gap-2 border-t border-border pt-4">
            <button type="button" onClick={close} className={buttonClass}>Batal</button>
            <button type="submit" disabled={busy || !auth?.verified || handle.length < 2} className={`${buttonClass} bg-primary text-primary-foreground hover:bg-primary/90`}>{busy && <RefreshCw className="h-4 w-4 animate-spin" />}Tambah agen</button>
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
