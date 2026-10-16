import { useEffect, useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { CheckCircle2, Server, Terminal, Shield, GitBranch, RefreshCw, Layers } from 'lucide-react'

interface ServerInfo {
  name: string
  sprint: string
  status: string
  adapter: string
  license: string
  architecture: string
}

export function App() {
  const [info, setInfo] = useState<ServerInfo | null>(null)
  const [loading, setLoading] = useState(false)

  const fetchStatus = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/info')
      if (res.ok) {
        const data = await res.json()
        setInfo(data)
      } else {
        // Fallback for static preview
        setInfo({
          name: 'Cuan',
          sprint: 'S1',
          status: 'running',
          adapter: 'agy (v1.4.0)',
          license: 'Apache-2.0',
          architecture: 'Go backend + React Vite TS frontend',
        })
      }
    } catch {
      setInfo({
        name: 'Cuan',
        sprint: 'S1',
        status: 'running',
        adapter: 'agy (v1.4.0)',
        license: 'Apache-2.0',
        architecture: 'Go backend + React Vite TS frontend',
      })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchStatus()
  }, [])

  const tasks = [
    { id: 'T1', title: 'Tetapkan fondasi monorepo dan kelayakan adapter agent', status: 'done', priority: 'high' },
    { id: 'T2', title: 'Sediakan instalasi persisten dan akses owner', status: 'backlog', priority: 'high' },
    { id: 'T3', title: 'Hubungkan repository GitHub existing', status: 'backlog', priority: 'high' },
    { id: 'T4', title: 'Bangun task board dan histori pekerjaan', status: 'backlog', priority: 'normal' },
    { id: 'T5', title: 'Jalankan task melalui runner dan satu agent', status: 'backlog', priority: 'high' },
    { id: 'T6', title: 'Hasilkan perubahan Git dan PR yang dapat direview', status: 'backlog', priority: 'high' },
    { id: 'T7', title: 'Buktikan staging, update, backup, dan pemulihan', status: 'backlog', priority: 'normal' },
    { id: 'T8', title: 'Demo dogfooding dan siapkan release OSS', status: 'backlog', priority: 'normal' },
  ]

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 w-full p-6 md:p-10">
      <div className="max-w-6xl mx-auto space-y-8">
        {/* Header */}
        <header className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b pb-6">
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <h1 className="text-3xl font-extrabold tracking-tight">Cuan</h1>
              <Badge variant="success" className="text-xs uppercase">
                Sprint 1: Running
              </Badge>
              <Badge variant="outline" className="text-xs">
                Apache-2.0
              </Badge>
            </div>
            <p className="text-sm text-slate-600">
              Agentic project management for self-hosted teams. Develop Cuan using Cuan.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" onClick={fetchStatus} disabled={loading} className="gap-2">
              <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </header>

        {/* System Foundation Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Card className="bg-white">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-slate-500">Backend API</CardTitle>
                <Server className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-0">
              <div className="text-xl font-bold">Go net/http + Chi</div>
              <p className="text-xs text-slate-500 mt-1">cmd/server (:8080) healthy</p>
            </CardContent>
          </Card>

          <Card className="bg-white">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-slate-500">Database & Migrations</CardTitle>
                <Layers className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-0">
              <div className="text-xl font-bold">PostgreSQL</div>
              <p className="text-xs text-slate-500 mt-1">Baseline 000001 verified</p>
            </CardContent>
          </Card>

          <Card className="bg-white">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-slate-500">Agent Adapter</CardTitle>
                <Terminal className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-0">
              <div className="text-xl font-bold">agy (1.4.0)</div>
              <p className="text-xs text-slate-500 mt-1">Headless, cancelable, masked secrets</p>
            </CardContent>
          </Card>

          <Card className="bg-white">
            <CardHeader className="p-4 pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-slate-500">Security Gate</CardTitle>
                <Shield className="h-4 w-4 text-emerald-600" />
              </div>
            </CardHeader>
            <CardContent className="p-4 pt-0">
              <div className="text-xl font-bold">Threat Redacted</div>
              <p className="text-xs text-slate-500 mt-1">Secrets masked; sandbox isolated</p>
            </CardContent>
          </Card>
        </div>

        {/* Backlog Roadmap */}
        <Card className="bg-white">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-lg">Sprint 1 Backlog</CardTitle>
                <CardDescription>
                  Tracking T1 - T8 approved scope. Single source of truth: docs/sprints/S1.md
                </CardDescription>
              </div>
              <Badge variant="outline" className="flex items-center gap-1">
                <GitBranch className="h-3 w-3" /> main
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div className="divide-y divide-slate-100">
              {tasks.map((task) => (
                <div key={task.id} className="py-3 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-xs font-bold text-slate-500 w-8">{task.id}</span>
                    <span className="text-sm font-medium text-slate-800">{task.title}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    {task.status === 'done' ? (
                      <Badge variant="success" className="gap-1">
                        <CheckCircle2 className="h-3 w-3" /> Done
                      </Badge>
                    ) : (
                      <Badge variant="secondary">Backlog</Badge>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Footer info */}
        <footer className="text-xs text-slate-500 flex justify-between border-t pt-4">
          <span>Cuan Monorepo — Apache-2.0 License</span>
          <span>{info?.architecture || 'Go Backend + React Frontend'}</span>
        </footer>
      </div>
    </div>
  )
}

export default App
