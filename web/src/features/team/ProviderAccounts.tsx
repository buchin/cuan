import { useEffect, useRef, useState } from 'react'
import { accountsApi, type ProviderAccount } from '../../api/accounts'
import type { HarnessAuth } from '../../api/agents'

const states: Record<string, string> = { unknown: 'Belum diverifikasi', verified: 'Terverifikasi', login_required: 'Perlu login', quota_limited: 'Kuota dibatasi', unavailable: 'Provider belum tersedia' }
export function ProviderAccounts() {
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [label, setLabel] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [session, setSession] = useState<(HarnessAuth & { account: ProviderAccount }) | null>(null)
  const [code, setCode] = useState('')
  const active = useRef<{ accountId: string; sessionId: string } | null>(null)
  const mounted = useRef(true)
  const load = async () => { try { setAccounts(await accountsApi.list()) } catch { setError('Daftar akun gagal dimuat. Coba lagi.') } finally { setLoading(false) } }
  useEffect(() => { mounted.current = true; void load(); return () => { mounted.current = false; const s = active.current; if (s) void accountsApi.cancel(s.accountId, s.sessionId).catch(() => {}) } }, [])
  const cancel = async () => {
    const s = active.current; active.current = null
    setSession(null); setCode('')
    if (s) await accountsApi.cancel(s.accountId, s.sessionId).catch(() => {})
  }
  useEffect(() => {
    if (!session?.expires_at) return
    const timer = window.setTimeout(() => { void cancel(); setError('Sesi login kedaluwarsa. Mulai login lagi pada akun tersebut.') }, Math.max(0, Date.parse(session.expires_at) - Date.now()))
    return () => clearTimeout(timer)
  }, [session?.expires_at])
  const run = async (action: () => Promise<void>) => {
    setError(''); setBusy(true)
    try { await action() } catch (e) { if (mounted.current) setError(e instanceof Error ? e.message : 'Permintaan gagal. Coba lagi.') }
    finally { if (mounted.current) { setBusy(false); await load() } }
  }
  const login = async (account: ProviderAccount) => {
    await cancel()
    const auth = await accountsApi.login(account.id)
    if (!auth.session_id || !auth.auth_url) throw new Error('Link login belum tersedia. Coba lagi.')
    if (!mounted.current) { await accountsApi.cancel(account.id, auth.session_id); return }
    active.current = { accountId: account.id, sessionId: auth.session_id }
    setSession({ ...auth, account })
  }
  const button = 'rounded-md border border-border px-3 py-2 text-sm hover:bg-muted disabled:opacity-50'
  return <section className="space-y-3 rounded-lg border border-border p-4" aria-labelledby="provider-accounts-title">
    <h2 id="provider-accounts-title" className="font-semibold">Akun provider</h2>
    <p className="text-sm text-muted-foreground">Hubungkan akun Google lain tanpa mengganti akun lama. Nama akun adalah label Anda; identitas Google belum ditampilkan. Agen yang memakai akun sama berbagi kuota.</p>
    <p className="text-xs text-muted-foreground">Tambah agen masih memakai akun server lama pada pratinjau ini.</p>
    <form onSubmit={e => { e.preventDefault(); void run(async () => { const a = await accountsApi.create(label); setLabel(''); await load(); await login(a) }) }} className="flex flex-col sm:flex-row gap-2">
      <label className="sr-only" htmlFor="account-label">Nama akun baru</label>
      <input id="account-label" value={label} onChange={e => setLabel(e.target.value)} maxLength={100} required placeholder="Nama akun baru, mis. Google kerja" className="min-w-0 flex-1 rounded-md border border-input bg-background px-3 py-2 text-base md:text-sm" />
      <button disabled={busy || !label.trim()} className={button}>Tambah akun Google</button>
    </form>
    {loading && <p role="status" className="text-sm">Memuat akun…</p>}
    {error && <p role="alert" className="text-sm text-destructive">{error} <button onClick={() => void load()} className="underline">Muat ulang</button></p>}
    {!loading && accounts.length === 0 && <p className="text-sm text-muted-foreground">Belum ada akun provider. Tambahkan akun untuk memulai.</p>}
    <ul className="divide-y divide-border">{accounts.map(a => <li key={a.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
      <div className="min-w-0"><p className="break-words text-sm font-medium">{a.label}</p><p className="text-xs text-muted-foreground">{states[a.state]} · {a.credentials_present ? 'Kredensial tersimpan' : 'Belum ada kredensial'}</p>{a.message && <p className="max-w-xl text-xs text-muted-foreground">{a.message}</p>}{a.last_checked_at && <p className="text-xs text-muted-foreground">Diperiksa {new Date(a.last_checked_at).toLocaleString()}</p>}</div>
      <button disabled={busy} onClick={() => void run(async () => { if (a.credentials_present || a.legacy) await accountsApi.verify(a.id); else await login(a) })} className={button}>{a.credentials_present || a.legacy ? 'Verifikasi akun' : 'Login Google'}</button>
    </li>)}</ul>
    {session && <div className="space-y-3 rounded-md border border-border p-3" role="region" aria-label="Login akun Google">
      <p className="text-sm font-medium">Login {session.account.label}</p>
      <a href={session.auth_url} target="_blank" rel="noopener noreferrer" className="block text-sm underline">Buka login Google ↗</a>
      <p className="text-xs text-muted-foreground">Pilih akun Google yang ingin ditambahkan. Selesaikan consent, lalu tempel kode di sini, bukan di chat. Sesi berlaku 5 menit.</p>
      <label htmlFor="new-account-code" className="block text-sm">Kode otorisasi</label>
      <input id="new-account-code" type="password" autoComplete="off" value={code} onChange={e => setCode(e.target.value)} className="w-full rounded-md border border-input bg-background px-3 py-2 text-base md:text-sm" />
      <div className="flex flex-wrap gap-2"><button disabled={busy || !code.trim()} onClick={() => void run(async () => { const s = active.current; if (!s) return; await accountsApi.code(s.accountId, s.sessionId, code); active.current = null; setCode(''); setSession(null) })} className={button}>Simpan dan verifikasi login</button><button disabled={busy} onClick={() => void cancel()} className={button}>Batalkan login</button></div>
    </div>}
    {busy && <p role="status" className="text-sm">Memproses akun…</p>}
  </section>
}
