import { useState, useEffect } from 'react'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import {
  Rocket,
  ExternalLink,
  Terminal as TerminalIcon,
  RotateCcw,
  Database,
  KeyRound,
  Globe,
  Layers,
  ShieldCheck
} from 'lucide-react'
import {
  ScriptDeploymentDTO,
  ProvisionedDatabaseDTO,
  SupervisedProcessStatus,
  ScriptDeployStatusResponse,
  ProjectItemDTO,
  ServerItem
} from '../types'
import { ScriptDeployerModal } from './ScriptDeployerModal'
import { ScriptDeployerConsole } from './ScriptDeployerConsole'

export interface ScriptDeployerViewProps {
  projectId: string
  projects?: ProjectItemDTO[]
  servers?: ServerItem[]
  baseDomain?: string
  onToast?: (type: 'success' | 'error', message: string) => void
  onProjectChange?: (projectId: string) => void
}

export function ScriptDeployerView({
  projectId: propProjectId,
  projects = [],
  servers = [],
  baseDomain = 'cuan.dojo.cc',
  onToast,
  onProjectChange,
}: ScriptDeployerViewProps) {
  const [selectedProjectId, setSelectedProjectId] = useState<string>(
    propProjectId || (projects.length > 0 ? projects[0].id : 'default')
  )

  useEffect(() => {
    if (propProjectId && propProjectId !== selectedProjectId) {
      setSelectedProjectId(propProjectId)
    }
  }, [propProjectId])

  const activeProject = projects.find((p) => p.id === selectedProjectId) || {
    id: selectedProjectId,
    name: selectedProjectId,
  }

  // Deployer state
  const [deployment, setDeployment] = useState<ScriptDeploymentDTO | null>(null)
  const [process, setProcess] = useState<SupervisedProcessStatus | null>(null)
  const [provisionedDb, setProvisionedDb] = useState<ProvisionedDatabaseDTO | null>(null)
  const [envCount, setEnvCount] = useState<number>(0)
  const [restarting, setRestarting] = useState(false)

  // Modal Wizard State
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [modalInitialStep, setModalInitialStep] = useState<1 | 2 | 3 | 4>(1)

  // Fetch status & details
  const fetchDeployerData = async () => {
    if (!selectedProjectId || selectedProjectId === 'default') return
    try {
      // 1. Script deploy status
      const resStatus = await fetch(`/api/v1/projects/${selectedProjectId}/script-deploy/status`)
      if (resStatus.ok) {
        const data: ScriptDeployStatusResponse = await resStatus.json()
        setDeployment(data.deployment || null)
        setProcess(data.process || null)
      } else {
        setDeployment(null)
        setProcess(null)
      }

      // 2. Env keys count
      const resEnv = await fetch(`/api/v1/projects/${selectedProjectId}/env`)
      if (resEnv.ok) {
        const dataEnv = await resEnv.json()
        setEnvCount(dataEnv.env_keys?.length || 0)
      }

      // 3. Provisioned database
      const resDb = await fetch(`/api/v1/projects/${selectedProjectId}/provision-db`)
      if (resDb.ok) {
        const dataDb = await resDb.json()
        if (dataDb.database) setProvisionedDb(dataDb.database)
      }
    } catch {}
  }

  useEffect(() => {
    fetchDeployerData()
    const timer = setInterval(fetchDeployerData, 10000)
    return () => clearInterval(timer)
  }, [selectedProjectId])

  const handleOpenWizard = (step: 1 | 2 | 3 | 4 = 1) => {
    setModalInitialStep(step)
    setIsModalOpen(true)
  }

  const handleRestart = async () => {
    setRestarting(true)
    try {
      const res = await fetch(`/api/v1/projects/${selectedProjectId}/script-deploy/restart`, {
        method: 'POST',
      })
      if (res.ok) {
        const data = await res.json()
        if (data.deployment) setDeployment(data.deployment)
        if (data.process) setProcess(data.process)
        onToast?.('success', '⚡ Service supervisor berhasil di-restart.')
      } else {
        const err = await res.json().catch(() => ({}))
        onToast?.('error', err.error || 'Gagal me-restart service.')
      }
    } catch (err: any) {
      onToast?.('error', err.message || 'Gagal me-restart service.')
    } finally {
      setRestarting(false)
    }
  }

  const isBuilding = deployment?.build_status === 'building' || process?.status === 'building'
  const isHealthy = process?.health === 'healthy' || deployment?.health === 'healthy'
  const isRunning = process?.status === 'running' || deployment?.build_status === 'ready'
  const isFailed = deployment?.build_status === 'failed' || process?.status === 'crashed'

  const liveUrl =
    (deployment?.domain ? `https://${deployment.domain}` : '') ||
    (process?.domain ? `https://${process.domain}` : '') ||
    (deployment?.port ? `http://127.0.0.1:${deployment.port}` : '')

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 p-5 bg-white dark:bg-slate-900 rounded-3xl border border-slate-200 dark:border-slate-800 shadow-xs">
        <div className="flex items-center gap-3.5">
          <div className="h-12 w-12 rounded-2xl bg-gradient-to-tr from-cyan-600 to-blue-600 text-white flex items-center justify-center shadow-lg shadow-cyan-500/20 shrink-0">
            <Rocket className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-bold text-slate-900 dark:text-white tracking-tight">
                1-Click Script Deployer &amp; Live Console
              </h1>
              <Badge className="bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20 text-[10px]">
                Sprint 12
              </Badge>
            </div>
            <p className="text-xs text-slate-400">
              Automated database provisioning, AES-256-GCM vault, build runner pipeline, dan supervisor process supervisor.
            </p>
          </div>
        </div>

        {/* Project Selector & Main Action */}
        <div className="flex flex-wrap items-center gap-2.5 w-full sm:w-auto justify-between sm:justify-end">
          {projects.length > 0 && (
            <div className="flex items-center gap-1.5">
              <Layers className="h-3.5 w-3.5 text-slate-400 shrink-0" />
              <select
                value={selectedProjectId}
                onChange={(e) => {
                  setSelectedProjectId(e.target.value)
                  onProjectChange?.(e.target.value)
                }}
                className="text-xs py-1.5 px-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 font-semibold text-slate-800 dark:text-slate-200 focus:outline-none focus:ring-1 focus:ring-cyan-500 max-w-[200px] truncate"
              >
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          <Button
            onClick={() => handleOpenWizard(1)}
            className="h-9 px-4 text-xs font-bold bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white gap-2 shadow-md shadow-cyan-600/20 rounded-xl"
          >
            <Rocket className="h-4 w-4" />
            <span>Launch Deployer Wizard</span>
          </Button>
        </div>
      </div>

      {/* Deployer Status & Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Card 1: Live Service Status */}
        <div className="p-5 rounded-3xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs flex flex-col justify-between space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
              <Globe className="h-4 w-4 text-cyan-500" />
              <span>Service Status</span>
            </span>
            {isBuilding ? (
              <Badge className="bg-amber-500/20 text-amber-500 border-amber-500/30 animate-pulse text-[11px]">
                Building...
              </Badge>
            ) : isHealthy && isRunning ? (
              <Badge className="bg-emerald-500/20 text-emerald-500 border-emerald-500/30 text-[11px] gap-1">
                <span className="h-2 w-2 rounded-full bg-emerald-500 animate-ping" />
                <span>Healthy (200 OK)</span>
              </Badge>
            ) : isRunning ? (
              <Badge className="bg-blue-500/20 text-blue-500 border-blue-500/30 text-[11px]">
                Running
              </Badge>
            ) : isFailed ? (
              <Badge className="bg-rose-500/20 text-rose-500 border-rose-500/30 text-[11px]">
                Failed
              </Badge>
            ) : (
              <Badge variant="outline" className="text-slate-400 text-[11px]">
                Not Deployed
              </Badge>
            )}
          </div>

          <div>
            <div className="text-xl font-extrabold text-slate-900 dark:text-white truncate">
              {deployment?.domain || process?.domain || 'Belum Dikonfigurasi'}
            </div>
            <p className="text-xs text-slate-400 font-mono mt-0.5">
              Port: {deployment?.port || process?.port || 'N/A'} • PID: {process?.pid || 'N/A'}
            </p>
          </div>

          <div className="flex items-center gap-2 pt-1">
            {liveUrl && (isRunning || isHealthy) ? (
              <a
                href={liveUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-xs font-semibold px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white transition-colors"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                <span>Open Live App</span>
              </a>
            ) : (
              <Button
                size="sm"
                variant="outline"
                onClick={() => handleOpenWizard(3)}
                className="text-xs gap-1 border-slate-300 dark:border-slate-700"
              >
                <Rocket className="h-3.5 w-3.5 text-cyan-500" />
                <span>Setup Domain</span>
              </Button>
            )}

            {isRunning && (
              <Button
                size="sm"
                variant="outline"
                onClick={handleRestart}
                disabled={restarting}
                className="text-xs gap-1 border-slate-300 dark:border-slate-700"
              >
                <RotateCcw className={`h-3 w-3 ${restarting ? 'animate-spin' : ''}`} />
                <span>Restart</span>
              </Button>
            )}
          </div>
        </div>

        {/* Card 2: Database Provisioning Status */}
        <div className="p-5 rounded-3xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs flex flex-col justify-between space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
              <Database className="h-4 w-4 text-purple-500" />
              <span>Isolated Database</span>
            </span>
            <Badge variant="outline" className="text-[10px]">
              {provisionedDb ? provisionedDb.engine.toUpperCase() : 'SQLite Default'}
            </Badge>
          </div>

          <div>
            <div className="text-xl font-extrabold text-slate-900 dark:text-white truncate">
              {provisionedDb?.db_name || 'cuan_isolated_db'}
            </div>
            <p className="text-xs text-slate-400 font-mono mt-0.5">
              Status: {provisionedDb?.status || 'Siap Diprovision'} • Schema:{' '}
              {provisionedDb?.schema_imported ? 'Imported' : 'Pending'}
            </p>
          </div>

          <div className="pt-1">
            <Button
              size="sm"
              variant="outline"
              onClick={() => handleOpenWizard(2)}
              className="text-xs gap-1 border-slate-300 dark:border-slate-700"
            >
              <Database className="h-3.5 w-3.5 text-purple-500" />
              <span>{provisionedDb ? 'Re-provision DB' : 'Provision Database'}</span>
            </Button>
          </div>
        </div>

        {/* Card 3: AES-256-GCM Secrets Vault */}
        <div className="p-5 rounded-3xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs flex flex-col justify-between space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
              <ShieldCheck className="h-4 w-4 text-emerald-500" />
              <span>Secrets Vault</span>
            </span>
            <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 text-[10px]">
              AES-256-GCM
            </Badge>
          </div>

          <div>
            <div className="text-xl font-extrabold text-slate-900 dark:text-white">
              {envCount} Variabel
            </div>
            <p className="text-xs text-slate-400 mt-0.5">
              Zero plaintext leakage. Terenkripsi di data layer &amp; server vault.
            </p>
          </div>

          <div className="pt-1">
            <Button
              size="sm"
              variant="outline"
              onClick={() => handleOpenWizard(1)}
              className="text-xs gap-1 border-slate-300 dark:border-slate-700"
            >
              <KeyRound className="h-3.5 w-3.5 text-emerald-500" />
              <span>Kelola .env &amp; Secrets</span>
            </Button>
          </div>
        </div>
      </div>

      {/* Embedded Live Console Section */}
      <div className="space-y-3">
        <div className="flex items-center justify-between px-1">
          <h2 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <TerminalIcon className="h-4 w-4 text-cyan-500" />
            <span>Supervisor Real-Time Terminal &amp; Build Logs</span>
          </h2>
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleOpenWizard(4)}
            className="text-xs gap-1 border-slate-300 dark:border-slate-700"
          >
            <TerminalIcon className="h-3.5 w-3.5" />
            <span>Maximize Console</span>
          </Button>
        </div>

        <ScriptDeployerConsole
          projectId={selectedProjectId}
          deployment={deployment}
          process={process}
          isBuilding={isBuilding}
          onRestart={handleRestart}
          liveAppUrl={liveUrl}
        />
      </div>

      {/* Interactive Deployer Modal Wizard */}
      <ScriptDeployerModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        projectId={selectedProjectId}
        projectName={activeProject.name}
        projectSlug={(activeProject as any).slug || activeProject.name.toLowerCase().replace(/[^a-z0-9]/g, '-')}
        baseDomain={baseDomain}
        servers={servers}
        initialStep={modalInitialStep}
        onDeploymentSuccess={(dep) => {
          setDeployment(dep)
          fetchDeployerData()
        }}
      />
    </div>
  )
}
