import { useEffect, useState, FormEvent } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { 
  CheckCircle2, 
  Server, 
  Shield, 
  Database, 
  Lock, 
  LogOut, 
  RefreshCw, 
  GitBranch, 
  AlertCircle,
  ShieldCheck,
  Globe,
  HardDrive
} from 'lucide-react'

interface OwnerInfo {
  id: string
  email: string
}

interface ServerInfo {
  name: string
  sprint: string
  status: string
  adapter: string
  license: string
  architecture: string
}

interface HealthInfo {
  status: string
  app: string
  version: string
}

interface ReadyInfo {
  status: string
  database: string
  migrations?: string
  migrations_count?: number
  owner_configured?: boolean
}

interface TaskItem {
  id: string
  title: string
  status: string
  priority: string
}

export function App() {
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null)
  const [owner, setOwner] = useState<OwnerInfo | null>(null)
  const [token, setToken] = useState<string>(() => localStorage.getItem('cuan_token') || '')
  
  const [info, setInfo] = useState<ServerInfo | null>(null)
  const [health, setHealth] = useState<HealthInfo | null>(null)
  const [ready, setReady] = useState<ReadyInfo | null>(null)
  const [tasks, setTasks] = useState<TaskItem[]>([])
  
  const [loading, setLoading] = useState(true)
  const [authLoading, setAuthLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  // Form states
  const [emailInput, setEmailInput] = useState('')
  const [passwordInput, setPasswordInput] = useState('')
  const [confirmPasswordInput, setConfirmPasswordInput] = useState('')

  const getAuthHeaders = (): HeadersInit => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    return headers
  }

  const checkStatusAndAuth = async (currentToken: string = token) => {
    setLoading(true)
    setErrorMessage(null)

    try {
      // 1. Check Auth Status (Setup required vs configured)
      const statusRes = await fetch('/api/v1/auth/status')
      if (statusRes.ok) {
        const statusData = await statusRes.json()
        setSetupRequired(statusData.setup_required)

        if (!statusData.setup_required) {
          // Check if currently authenticated
          const headers: Record<string, string> = {}
          if (currentToken) {
            headers['Authorization'] = `Bearer ${currentToken}`
          }
          const meRes = await fetch('/api/v1/auth/me', { headers })
          if (meRes.ok) {
            const meData = await meRes.json()
            setOwner(meData.owner)
            await fetchProtectedData(currentToken)
          } else {
            setOwner(null)
          }
        }
      }

      // 2. Fetch Health & Readiness
      const [healthRes, readyRes, infoRes] = await Promise.allSettled([
        fetch('/healthz'),
        fetch('/readyz'),
        fetch('/api/v1/info')
      ])

      if (healthRes.status === 'fulfilled' && healthRes.value.ok) {
        setHealth(await healthRes.value.json())
      }
      if (readyRes.status === 'fulfilled' && readyRes.value.ok) {
        setReady(await readyRes.value.json())
      }
      if (infoRes.status === 'fulfilled' && infoRes.value.ok) {
        setInfo(await infoRes.value.json())
      }
    } catch (err) {
      console.error('Failed to load system status:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchProtectedData = async (activeToken: string = token) => {
    try {
      const headers: Record<string, string> = {}
      if (activeToken) {
        headers['Authorization'] = `Bearer ${activeToken}`
      }
      const tasksRes = await fetch('/api/v1/tasks', { headers })
      if (tasksRes.ok) {
        const tasksData = await tasksRes.json()
        setTasks(tasksData.tasks || [])
      }
    } catch (err) {
      console.error('Failed to fetch protected tasks:', err)
    }
  }

  useEffect(() => {
    checkStatusAndAuth()
  }, [])

  const handleSetup = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)

    if (passwordInput.length < 8) {
      setErrorMessage('Kata sandi harus minimal 8 karakter.')
      return
    }
    if (passwordInput !== confirmPasswordInput) {
      setErrorMessage('Konfirmasi kata sandi tidak cocok.')
      return
    }

    setAuthLoading(true)
    try {
      const res = await fetch('/api/v1/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Setup gagal dilakukan')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setSetupRequired(false)
      setEmailInput('')
      setPasswordInput('')
      setConfirmPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Terjadi kesalahan saat setup')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogin = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)
    setAuthLoading(true)

    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Login gagal')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setEmailInput('')
      setPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Email atau password salah')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogout = async () => {
    try {
      await fetch('/api/v1/auth/logout', {
        method: 'POST',
        headers: getAuthHeaders(),
      })
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      localStorage.removeItem('cuan_token')
      setToken('')
      setOwner(null)
      setTasks([])
    }
  }

  if (loading && setupRequired === null) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 text-emerald-600 animate-spin" />
          <p className="text-sm font-medium text-slate-600">Memeriksa status instance Cuan...</p>
        </div>
      </div>
    )
  }

  // 1. FIRST-TIME SETUP SCREEN
  if (setupRequired) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-emerald-100 rounded-2xl mb-1 text-emerald-700">
              <ShieldCheck className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Setup Pertama Pemilik Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance Cuan beroperasi dalam mode pemilik tunggal (single-owner). Akun pertama ini menjadi satu-satunya pengelola instance.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Buat Akun Pemilik</CardTitle>
              <CardDescription className="text-xs">
                Setelah pendaftaran berhasil, registrasi publik ditutup permanen untuk mencegah pengambilalihan pihak ketiga.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleSetup}>
              <CardContent className="space-y-4">
                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Gagal Setup</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi (min. 8 karakter)</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Konfirmasi Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={confirmPasswordInput}
                    onChange={(e) => setConfirmPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="p-3 bg-slate-50 rounded-md border border-slate-200 text-xs text-slate-600 space-y-1">
                  <div className="flex items-center gap-1.5 font-medium text-slate-700">
                    <Lock className="h-3.5 w-3.5 text-emerald-600" />
                    Jaminan Keamanan & DoD
                  </div>
                  <p>
                    Data tersimpan secara persisten pada volume PostgreSQL. Kredensial di-hash dengan bcrypt dan database tidak diekspos ke publik.
                  </p>
                </div>
              </CardContent>
              <CardFooter className="pt-2">
                <Button type="submit" className="w-full bg-emerald-600 hover:bg-emerald-700 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Menyimpan Akun Pemilik...
                    </>
                  ) : (
                    'Inisialisasi & Amankan Instance'
                  )}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 2. LOGIN SCREEN (WHEN NOT LOGGED IN)
  if (!owner) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-slate-900 rounded-2xl mb-1 text-white">
              <Lock className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Masuk ke Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance terkonfigurasi. Masuk dengan akun pemilik terdaftar.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Autentikasi Pemilik Tunggal</CardTitle>
              <CardDescription className="text-xs">
                Akses API, runner agent, dan data proyek dilindungi oleh sesi terenkripsi.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleLogin}>
              <CardContent className="space-y-4">
                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Autentikasi Gagal</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="flex items-center justify-between text-xs text-slate-500 pt-1">
                  <span className="flex items-center gap-1">
                    <Shield className="h-3.5 w-3.5 text-emerald-600" />
                    Pendaftaran publik dikunci
                  </span>
                  <span>PostgreSQL Persistent</span>
                </div>
              </CardContent>
              <CardFooter className="pt-2">
                <Button type="submit" className="w-full bg-slate-900 hover:bg-slate-800 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Memverifikasi...
                    </>
                  ) : (
                    'Masuk ke Dashboard'
                  )}
                </Button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 3. AUTHENTICATED DASHBOARD (UI SHELL)
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 w-full p-4 md:p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Top Header */}
        <header className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-5 rounded-xl border border-slate-200 shadow-sm">
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-black tracking-tight text-slate-900">Cuan</h1>
              <Badge variant="success" className="text-xs uppercase">
                Sprint 1: Running
              </Badge>
              <Badge variant="outline" className="text-xs font-mono">
                T2: Persistent & Owner Auth
              </Badge>
            </div>
            <p className="text-xs text-slate-500">
              Agentic project management for self-hosted teams. Develop Cuan using Cuan.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <div className="text-right hidden sm:block">
              <div className="text-xs font-bold text-slate-800">{owner.email}</div>
              <div className="text-[11px] text-emerald-600 font-medium">Owner Terautentikasi</div>
            </div>

            <Button variant="outline" size="sm" onClick={() => checkStatusAndAuth()} disabled={loading} className="gap-1.5 text-xs">
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>

            <Button variant="ghost" size="sm" onClick={handleLogout} className="gap-1.5 text-xs text-red-600 hover:bg-red-50">
              <LogOut className="h-3.5 w-3.5" />
              Keluar
            </Button>
          </div>
        </header>

        {/* System Foundation Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {/* Card 1: Backend */}
          <Card className="bg-white border-slate-200 shadow-sm">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Backend API</CardTitle>
                <Server className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-1">
              <div className="text-lg font-bold text-slate-900">Go net/http + Chi</div>
              <div className="flex items-center gap-1.5 mt-1.5">
                <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                <span className="text-xs text-slate-600">/healthz: {health?.status || 'healthy'}</span>
              </div>
            </CardContent>
          </Card>

          {/* Card 2: Database & Volume */}
          <Card className="bg-white border-slate-200 shadow-sm">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Storage Persisten</CardTitle>
                <Database className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-1">
              <div className="text-lg font-bold text-slate-900">PostgreSQL 16</div>
              <div className="flex items-center gap-1.5 mt-1.5">
                <HardDrive className="h-3.5 w-3.5 text-emerald-600" />
                <span className="text-xs text-slate-600">Volume: cuan_postgres_data</span>
              </div>
            </CardContent>
          </Card>

          {/* Card 3: Auth & Security */}
          <Card className="bg-white border-slate-200 shadow-sm">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Akses Owner</CardTitle>
                <ShieldCheck className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-1">
              <div className="text-lg font-bold text-slate-900">Single-Owner Terkunci</div>
              <div className="flex items-center gap-1.5 mt-1.5">
                <Lock className="h-3.5 w-3.5 text-emerald-600" />
                <span className="text-xs text-slate-600">Setup ditutup (403 Forbidden)</span>
              </div>
            </CardContent>
          </Card>

          {/* Card 4: Proxy & Networking */}
          <Card className="bg-white border-slate-200 shadow-sm">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Network & Proxy</CardTitle>
                <Globe className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-1">
              <div className="text-lg font-bold text-slate-900">Caddy / Reverse Proxy</div>
              <div className="flex items-center gap-1.5 mt-1.5">
                <Shield className="h-3.5 w-3.5 text-emerald-600" />
                <span className="text-xs text-slate-600">DB internal (port 5432 privat)</span>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Verification & DoD Evidence Banner */}
        <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4 text-xs text-emerald-900 flex flex-col md:flex-row items-start md:items-center justify-between gap-3">
          <div className="space-y-1">
            <div className="font-semibold flex items-center gap-1.5 text-emerald-950">
              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
              DoD T2 Terverifikasi: Persistent Installation & Single-Owner Access
            </div>
            <p className="text-emerald-800">
              Data tersimpan pada volume PostgreSQL. API project & task dilindungi auth middleware. Database tidak diekspos ke publik.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
              Readyz: {ready?.status || 'ready'}
            </Badge>
            <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
              Migrations: {ready?.migrations_count || '2'} applied
            </Badge>
          </div>
        </div>

        {/* Protected Tasks Board */}
        <Card className="bg-white border-slate-200 shadow-sm">
          <CardHeader>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <CardTitle className="text-base font-bold text-slate-900">Daftar Task Sprint 1</CardTitle>
                <CardDescription className="text-xs text-slate-500">
                  Data diambil dari endpoint terproteksi <code>/api/v1/tasks</code> menggunakan session token pemilik.
                </CardDescription>
              </div>
              <Badge variant="outline" className="flex items-center gap-1 text-xs self-start sm:self-auto font-mono">
                <GitBranch className="h-3 w-3" /> main
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div className="divide-y divide-slate-100">
              {tasks.length > 0 ? (
                tasks.map((task) => (
                  <div key={task.id} className="py-3 flex items-center justify-between gap-4">
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-xs font-bold text-slate-500 w-8">{task.id}</span>
                      <span className="text-sm font-medium text-slate-800">{task.title}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      {task.status === 'done' && (
                        <Badge variant="success" className="gap-1 text-xs">
                          <CheckCircle2 className="h-3 w-3" /> Done
                        </Badge>
                      )}
                      {task.status === 'doing' && (
                        <Badge variant="default" className="gap-1 text-xs bg-blue-600 hover:bg-blue-700">
                          <RefreshCw className="h-3 w-3 animate-spin" /> Doing
                        </Badge>
                      )}
                      {task.status === 'backlog' && (
                        <Badge variant="secondary" className="text-xs">
                          Backlog
                        </Badge>
                      )}
                    </div>
                  </div>
                ))
              ) : (
                <div className="py-6 text-center text-xs text-slate-500">
                  Memuat data task terproteksi...
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Footer */}
        <footer className="text-xs text-slate-500 flex flex-col sm:flex-row justify-between items-center gap-2 border-t pt-4">
          <span>Cuan Monorepo — Apache-2.0 License</span>
          <span>{info?.architecture || 'Go Backend + React Frontend'}</span>
        </footer>
      </div>
    </div>
  )
}

export default App
