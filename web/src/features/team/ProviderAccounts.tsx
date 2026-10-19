import { useEffect, useRef, useState } from 'react'
import { accountsApi, type ProviderAccount } from '../../api/accounts'

const states: Record<string, string> = { unknown: 'Perlu diperiksa', verified: 'Terverifikasi', login_required: 'Login belum selesai', quota_limited: 'Kuota habis', unavailable: 'Koneksi terganggu' }
export function ProviderAccounts({ onLogin }: { onLogin: (account: ProviderAccount) => void }) {
  const [accounts, setAccounts] = useState<ProviderAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const mounted = useRef(true)
  const load = async () => { try { const data = await accountsApi.list(); if (mounted.current) setAccounts(data) } catch { if (mounted.current) setError('Daftar akun belum bisa dibuka. Coba lagi.') } finally { if (mounted.current) setLoading(false) } }
  useEffect(() => { mounted.current = true; void load(); return () => { mounted.current = false } }, [])
  const check = async (id: string) => {
    setBusy(id); setError('')
    try { await accountsApi.verify(id); await load() }
    catch { if (mounted.current) setError('Koneksi belum bisa diperiksa. Akun Anda tetap tersimpan.') }
    finally { if (mounted.current) setBusy('') }
  }
  return <section aria-label="Akun Google tersimpan" className="space-y-3">
    <p className="text-xs text-muted-foreground">Akun Anda tetap tersimpan saat menambahkan agen lain. Agen dengan akun yang sama berbagi kuota.</p>
    {loading && <p role="status" className="text-sm">Memuat akun…</p>}
    {error && <p role="alert" className="text-sm text-destructive">{error} <button onClick={() => void load()} className="underline">Coba lagi</button></p>}
    {!loading && !accounts.length && <p className="text-sm text-muted-foreground">Belum ada akun. Gunakan Tambah agen untuk menghubungkan Google.</p>}
    <ul className="divide-y divide-border">{accounts.map(a => <li key={a.id} className="flex flex-wrap items-center justify-between gap-3 py-3"><div className="min-w-0 flex-1"><p className="break-words text-sm font-medium">{a.label}</p><p className="mt-1 text-xs text-muted-foreground">{states[a.state]}</p>{a.state === 'quota_limited' && <p className="mt-1 text-xs text-muted-foreground">Tunggu kuota pulih, lalu periksa lagi.</p>}</div><button disabled={!!busy} onClick={() => a.credentials_present || a.legacy ? void check(a.id) : onLogin(a)} className="shrink-0 rounded-md border border-border px-3 py-2 text-sm hover:bg-muted disabled:opacity-50">{busy === a.id ? 'Memeriksa…' : a.credentials_present || a.legacy ? 'Periksa koneksi' : 'Lanjutkan login'}</button></li>)}</ul>
  </section>
}
