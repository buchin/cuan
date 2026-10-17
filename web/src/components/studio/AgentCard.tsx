import { useState } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  Bot,
  Sparkles,
  Edit,
  Trash2,
  ChevronDown,
  ChevronUp,
  Cpu,
  Power
} from 'lucide-react'
import { AgentDTO } from './types'

interface AgentCardProps {
  agent: AgentDTO
  onEdit: (agent: AgentDTO) => void
  onDelete: (agentId: string) => void
  onToggleStatus?: (agent: AgentDTO, newStatus: string) => void
}

export function AgentCard({
  agent,
  onEdit,
  onDelete,
  onToggleStatus,
}: AgentCardProps) {
  const [expanded, setExpanded] = useState(false)

  // Status color helpers
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active':
        return (
          <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 text-[10px] py-0 px-2 font-mono flex items-center gap-1">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            Active
          </Badge>
        )
      case 'inactive':
        return (
          <Badge variant="outline" className="text-slate-500 border-slate-300 dark:border-slate-700 text-[10px] py-0 px-2 font-mono">
            Inactive
          </Badge>
        )
      case 'archived':
        return (
          <Badge variant="outline" className="text-amber-500 border-amber-300 dark:border-amber-700 text-[10px] py-0 px-2 font-mono">
            Archived
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="text-slate-500 text-[10px] py-0 px-2 font-mono">
            {status}
          </Badge>
        )
    }
  }

  // Model badge helper
  const getModelBadge = (modelId: string) => {
    if (modelId.includes('pro')) {
      return (
        <Badge className="bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/30 text-[10px] font-mono py-0 px-1.5">
          <Sparkles className="h-2.5 w-2.5 mr-1" />
          {modelId}
        </Badge>
      )
    }
    if (modelId.includes('flash')) {
      return (
        <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 text-[10px] font-mono py-0 px-1.5">
          <Cpu className="h-2.5 w-2.5 mr-1" />
          {modelId}
        </Badge>
      )
    }
    return (
      <Badge variant="outline" className="text-slate-600 dark:text-slate-400 text-[10px] font-mono py-0 px-1.5">
        {modelId}
      </Badge>
    )
  }

  return (
    <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-all rounded-2xl flex flex-col justify-between group">
      {/* Top Bar: Avatar, Name, Role, Badges */}
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-3 min-w-0">
            {/* Avatar */}
            <div className="h-11 w-11 rounded-xl bg-gradient-to-br from-emerald-500/20 to-teal-600/20 border border-emerald-500/30 text-emerald-500 flex items-center justify-center text-xl shrink-0 shadow-xs">
              {agent.avatar ? (
                <span>{agent.avatar}</span>
              ) : (
                <Bot className="h-5 w-5 text-emerald-500" />
              )}
            </div>

            {/* Name & Role */}
            <div className="min-w-0">
              <h3 className="text-sm font-bold text-slate-900 dark:text-white truncate">
                {agent.name}
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400 truncate">
                {agent.role || 'Autonomous Engineering Agent'}
              </p>
            </div>
          </div>

          {/* Status Badge */}
          <div className="shrink-0">{getStatusBadge(agent.status)}</div>
        </div>

        {/* Tags / Models Bar */}
        <div className="flex items-center gap-1.5 flex-wrap pt-1 border-t border-slate-100 dark:border-slate-800/80">
          {getModelBadge(agent.model_id || 'gemini-1.5-pro')}
          <Badge variant="outline" className="text-[10px] font-mono text-slate-500 py-0 px-1.5">
            {agent.provider || 'google'}
          </Badge>
        </div>

        {/* System Prompt Collapsible Preview */}
        {agent.system_prompt && (
          <div className="space-y-1">
            <button
              type="button"
              onClick={() => setExpanded(!expanded)}
              className="flex items-center justify-between w-full text-[11px] font-semibold text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 py-0.5"
            >
              <span>System Prompt</span>
              <span className="flex items-center gap-0.5 text-slate-400">
                {expanded ? (
                  <>
                    <ChevronUp className="h-3 w-3" /> Tutup
                  </>
                ) : (
                  <>
                    <ChevronDown className="h-3 w-3" /> Lihat
                  </>
                )}
              </span>
            </button>

            {expanded ? (
              <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-[11px] font-mono text-slate-600 dark:text-slate-300 whitespace-pre-wrap max-h-48 overflow-y-auto leading-relaxed animate-in fade-in duration-150">
                {agent.system_prompt}
              </div>
            ) : (
              <p className="text-[11px] text-slate-400 line-clamp-2 italic">
                "{agent.system_prompt}"
              </p>
            )}
          </div>
        )}
      </div>

      {/* Footer: Date & Card Action Buttons */}
      <div className="pt-3 mt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-2 text-xs">
        <span className="text-[10px] text-slate-400 font-mono">
          ID: #{agent.id.slice(0, 8)}
        </span>

        <div className="flex items-center gap-1.5">
          {onToggleStatus && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onToggleStatus(agent, agent.status === 'active' ? 'inactive' : 'active')}
              className="h-7 px-2 text-[11px] text-slate-500 hover:text-slate-900 dark:hover:text-slate-100"
              title={agent.status === 'active' ? 'Nonaktifkan agen' : 'Aktifkan agen'}
            >
              <Power className="h-3 w-3" />
            </Button>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => onEdit(agent)}
            className="h-7 px-2.5 text-[11px] font-medium text-slate-700 dark:text-slate-200 gap-1"
          >
            <Edit className="h-3 w-3" />
            <span>Edit</span>
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => onDelete(agent.id)}
            className="h-7 px-2 text-[11px] text-slate-400 hover:text-red-500 dark:hover:text-red-400"
            title="Hapus Agen"
          >
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </div>
    </Card>
  )
}
