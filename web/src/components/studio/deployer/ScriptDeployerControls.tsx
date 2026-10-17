import { useState, useEffect } from 'react'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import {
  Rocket,
  ExternalLink,
  Terminal as TerminalIcon,
  RotateCcw,
  RefreshCw,
  CheckCircle2,
  AlertCircle
} from 'lucide-react'
import { ScriptDeploymentDTO, SupervisedProcessStatus, ScriptDeployStatusResponse } from '../types'

export interface ScriptDeployerControlsProps {
  projectId: string
  projectName?: string
  projectSlug?: string
  baseDomain?: string
  onOpenDeployerWizard: (initialStep?: 1 | 2 | 3 | 4) => void
  onToast?: (type: 'success' | 'error', message: string) => void
  className?: string
}

export function ScriptDeployerControls({
  projectId,
  projectName = 'Project',
  projectSlug = '',
  baseDomain = 'cuan.dojo.cc',
  onOpenDeployerWizard,
  onToast,
  className = '',
}: ScriptDeployerControlsProps) {
  const [deployment, setDeployment] = useState<ScriptDeploymentDTO | null>(null)
  const [process, setProcess] = useState<SupervisedProcessStatus | null>(null)
  const [restarting, setRestarting] = useState(false)

  // Fetch status
  const fetchStatus = async () => {
    if (!projectId || projectId === 'default') return
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/script-deploy/status`)
      if (res.ok) {
        const data: ScriptDeployStatusResponse = await res.json()
        if (data.deployment) setDeployment(data.deployment)
        if (data.process) setProcess(data.process)
      } else if (res.status === 404) {
        setDeployment(null)
        setProcess(null)
      }
    } catch {}
  }

  useEffect(() => {
    fetchStatus()
    const timer = setInterval(fetchStatus, 8000)
    return () => clearInterval(timer)
  }, [projectId])

  // Restart Handler
  const handleRestart = async () => {
    setRestarting(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/script-deploy/restart`, {
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

  // Derive status
  const isBuilding = deployment?.build_status === 'building' || process?.status === 'building'
  const isHealthy = process?.health === 'healthy' || deployment?.health === 'healthy'
  const isRunning = process?.status === 'running' || deployment?.build_status === 'ready'
  const isUnhealthy = (isRunning && (process?.health === 'unhealthy' || deployment?.health === 'unhealthy'))
  const isFailed = deployment?.build_status === 'failed' || process?.status === 'crashed'
  const isStopped = process?.status === 'stopped'

  // URL calculation
  const fallbackDomain = projectSlug ? `${projectSlug}.${baseDomain}` : ''
  const liveUrl =
    (deployment?.domain ? `https://${deployment.domain}` : '') ||
    (process?.domain ? `https://${process.domain}` : '') ||
    (fallbackDomain ? `https://${fallbackDomain}` : '') ||
    (deployment?.port ? `http://127.0.0.1:${deployment.port}` : '')

  return (
    <div className={`flex flex-wrap items-center gap-2 text-xs ${className}`}>
      {/* Status Badge */}
      {isBuilding ? (
        <Badge className="bg-amber-500/15 text-amber-500 border border-amber-500/30 gap-1 animate-pulse text-[11px] font-semibold">
          <RefreshCw className="h-3 w-3 animate-spin" />
          <span>Building</span>
        </Badge>
      ) : isHealthy && isRunning ? (
        <Badge className="bg-emerald-500/15 text-emerald-500 border border-emerald-500/30 gap-1 text-[11px] font-semibold">
          <span className="h-2 w-2 rounded-full bg-emerald-500 animate-ping" />
          <span>Healthy (200)</span>
        </Badge>
      ) : isUnhealthy ? (
        <Badge className="bg-amber-500/15 text-amber-500 border border-amber-500/30 gap-1 text-[11px] font-semibold">
          <AlertCircle className="h-3 w-3" />
          <span>Unhealthy</span>
        </Badge>
      ) : isRunning ? (
        <Badge className="bg-blue-500/15 text-blue-500 border border-blue-500/30 gap-1 text-[11px] font-semibold">
          <CheckCircle2 className="h-3 w-3" />
          <span>Running</span>
        </Badge>
      ) : isFailed ? (
        <Badge className="bg-rose-500/15 text-rose-500 border border-rose-500/30 gap-1 text-[11px] font-semibold">
          <AlertCircle className="h-3 w-3" />
          <span>Failed</span>
        </Badge>
      ) : isStopped ? (
        <Badge variant="outline" className="border-slate-300 dark:border-slate-700 text-slate-400 text-[11px]">
          <span>Stopped</span>
        </Badge>
      ) : (
        <Badge variant="outline" className="border-slate-300 dark:border-slate-700 text-slate-400 text-[11px]">
          <span>Not Deployed</span>
        </Badge>
      )}

      {/* Action: Deploy Script */}
      <Button
        variant="outline"
        size="sm"
        onClick={() => onOpenDeployerWizard(1)}
        className="h-8 text-xs font-semibold gap-1.5 border-cyan-500/30 bg-cyan-500/5 hover:bg-cyan-500/10 text-cyan-600 dark:text-cyan-400"
        title={`Buka 1-Click Script Deployer Wizard untuk ${projectName}`}
      >
        <Rocket className="h-3.5 w-3.5" />
        <span>Deploy Script</span>
      </Button>

      {/* Action: Open Live App */}
      {liveUrl && (isRunning || isHealthy) && (
        <a
          href={liveUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 h-8 px-2.5 rounded-lg text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white transition-colors shadow-xs"
          title={`Buka aplikasi live di ${liveUrl}`}
        >
          <ExternalLink className="h-3.5 w-3.5" />
          <span>Open Live App</span>
        </a>
      )}

      {/* Action: View Logs */}
      <Button
        variant="outline"
        size="sm"
        onClick={() => onOpenDeployerWizard(4)}
        className="h-8 text-xs gap-1 border-slate-300 dark:border-slate-700"
        title="Lihat live build logs"
      >
        <TerminalIcon className="h-3.5 w-3.5 text-slate-500" />
        <span className="hidden sm:inline">View Logs</span>
      </Button>

      {/* Action: Restart Service */}
      {isRunning && (
        <Button
          variant="outline"
          size="sm"
          onClick={handleRestart}
          disabled={restarting}
          className="h-8 text-xs gap-1 border-slate-300 dark:border-slate-700"
          title="Restart service background supervisor"
        >
          <RotateCcw className={`h-3.5 w-3.5 text-slate-500 ${restarting ? 'animate-spin' : ''}`} />
          <span className="hidden sm:inline">Restart</span>
        </Button>
      )}
    </div>
  )
}
