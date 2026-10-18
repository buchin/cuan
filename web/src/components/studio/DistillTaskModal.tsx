import { useState, useEffect, useRef } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import {
  X,
  Sparkles,
  AlertCircle,
  Plus,
  Trash2,
  ShieldCheck,
  Check,
  RefreshCw,
  ListTodo,
  FolderGit2,
  ChevronDown
} from 'lucide-react'
import { DoDItem, ProjectTaskDTO, DistillResult } from './types'
import { cn } from '../../lib/utils'

interface DistillTaskModalProps {
  isOpen: boolean
  onClose: () => void
  projectId: string
  roomId?: string
  roomTitle?: string
  selectedMessageIds?: string[]
  selectedMessageContent?: string
  availableProjects?: { id: string; name: string; is_archived?: boolean; status?: string }[]
  onTaskCreated?: (task: ProjectTaskDTO) => void
  initialTask?: ProjectTaskDTO | null
  onTaskUpdated?: (task: ProjectTaskDTO) => void
}

export function DistillTaskModal({
  isOpen,
  onClose,
  projectId,
  roomId,
  roomTitle,
  selectedMessageIds = [],
  selectedMessageContent,
  availableProjects,
  onTaskCreated,
  initialTask,
  onTaskUpdated,
}: DistillTaskModalProps) {
  if (!isOpen) return null

  const isEditMode = !!initialTask

  // Form states
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState<'low' | 'normal' | 'high' | 'urgent'>('normal')
  const [status, setStatus] = useState<'backlog' | 'doing' | 'review' | 'done'>('backlog')
  const [batchId, setBatchId] = useState('')
  const [parentTaskId, setParentTaskId] = useState('')

  // Multi-Project Linking States
  const [activeProjectsList, setActiveProjectsList] = useState<{ id: string; name: string }[]>([])
  const [selectedProjectIds, setSelectedProjectIds] = useState<string[]>([])
  const [isProjectDropdownOpen, setIsProjectDropdownOpen] = useState(false)
  
  // Tags
  const [tags, setTags] = useState<string[]>([])
  const [tagInput, setTagInput] = useState('')

  // DoD Checklist
  const [dodItems, setDodItems] = useState<DoDItem[]>([
    { text: 'Kode lolos lint & typecheck TypeScript (npx tsc --noEmit)', completed: false },
    { text: 'Targeted automated test lulus 100%', completed: false },
    { text: 'Audit trail terdokumentasi dan siap deploy', completed: false }
  ])
  const [newDodText, setNewDodText] = useState('')

  // Status & loading
  const [loading, setLoading] = useState(false)
  const [autoDistilling, setAutoDistilling] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [distillStats, setDistillStats] = useState<{ explicit: number; inferred: number } | null>(null)

  // Initialize draft when opened
  useEffect(() => {
    if (isOpen) {
      if (initialTask) {
        setTitle(initialTask.title || '')
        setDescription(initialTask.description || '')
        setPriority((initialTask.priority as any) || 'normal')
        setStatus((initialTask.status as any) || 'backlog')
        setBatchId(initialTask.batch_id || '')
        setParentTaskId(initialTask.parent_task_id || '')
        setTags(initialTask.tags || [])
        setDodItems(
          initialTask.dod_checklist && initialTask.dod_checklist.length > 0
            ? initialTask.dod_checklist
            : [
                { text: 'Kode lolos lint & typecheck TypeScript (npx tsc --noEmit)', completed: false },
                { text: 'Targeted automated test lulus 100%', completed: false },
                { text: 'Audit trail terdokumentasi dan siap deploy', completed: false },
              ]
        )
        const pids =
          initialTask.project_ids && initialTask.project_ids.length > 0
            ? initialTask.project_ids
            : initialTask.project_id
            ? [initialTask.project_id]
            : []
        setSelectedProjectIds(pids)
      } else {
        if (selectedMessageContent) {
          // Pre-populate description or title
          const lines = selectedMessageContent.split('\n').filter(Boolean)
          if (lines.length > 0) {
            const firstLine = lines[0].replace(/^[#\-*\s]+/, '').trim()
            setTitle(firstLine.slice(0, 80))
            setDescription(selectedMessageContent)
          }
        } else if (roomTitle) {
          setTitle(`Task dari: ${roomTitle}`)
        } else {
          setTitle('')
          setDescription('')
        }
        setPriority('normal')
        setStatus('backlog')
        setBatchId('')
        setParentTaskId('')
        setTags([])
        setDodItems([
          { text: 'Kode lolos lint & typecheck TypeScript (npx tsc --noEmit)', completed: false },
          { text: 'Targeted automated test lulus 100%', completed: false },
          { text: 'Audit trail terdokumentasi dan siap deploy', completed: false },
        ])
        if (projectId) {
          setSelectedProjectIds([projectId])
        } else {
          setSelectedProjectIds([])
        }
      }
    }
  }, [isOpen, initialTask, selectedMessageContent, roomTitle, projectId])

  const projectDropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (projectDropdownRef.current && !projectDropdownRef.current.contains(e.target as Node)) {
        setIsProjectDropdownOpen(false)
      }
    }
    if (isProjectDropdownOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isProjectDropdownOpen])

  // Load active projects for multi-project linking
  useEffect(() => {
    if (isOpen) {
      if (availableProjects && availableProjects.length > 0) {
        const active = availableProjects.filter((p) => !p.is_archived && p.status !== 'archived')
        setActiveProjectsList(active)
      } else {
        fetch('/api/v1/projects')
          .then((res) => res.json())
          .then((data) => {
            if (data && Array.isArray(data.projects)) {
              setActiveProjectsList(data.projects)
            }
          })
          .catch(() => {})
      }
    }
  }, [isOpen, availableProjects])

  const toggleProjectSelection = (id: string) => {
    if (selectedProjectIds.includes(id)) {
      setSelectedProjectIds(selectedProjectIds.filter((pid) => pid !== id))
    } else {
      setSelectedProjectIds([...selectedProjectIds, id])
    }
  }

  // Handle Tag Addition
  const handleAddTag = (t: string) => {
    const trimmed = t.trim().toLowerCase().replace(/^[#\s]+/, '')
    if (trimmed && !tags.includes(trimmed)) {
      setTags([...tags, trimmed])
    }
    setTagInput('')
  }

  const handleRemoveTag = (tagToRemove: string) => {
    setTags(tags.filter((t) => t !== tagToRemove))
  }

  // Handle DoD Operations
  const handleAddDoD = () => {
    const trimmed = newDodText.trim()
    if (!trimmed) return
    setDodItems([...dodItems, { text: trimmed, completed: false }])
    setNewDodText('')
  }

  const handleToggleDoD = (index: number) => {
    setDodItems(
      dodItems.map((item, idx) =>
        idx === index ? { ...item, completed: !item.completed } : item
      )
    )
  }

  const handleRemoveDoD = (index: number) => {
    setDodItems(dodItems.filter((_, idx) => idx !== index))
  }

  // 1-Click Auto-Distill via AI Distiller Engine API
  const handleAutoDistill = async () => {
    if (!roomId) {
      setError('Pilih Room diskusi terlebih dahulu untuk menjalankan auto-distill.')
      return
    }

    setAutoDistilling(true)
    setError(null)
    try {
      const payload: Record<string, any> = {
        title: title.trim() || undefined,
        batch_id: batchId.trim() || undefined,
        tags: tags.length > 0 ? tags : undefined,
      }
      if (selectedMessageIds.length > 0) {
        payload.message_ids = selectedMessageIds
      }

      const res = await fetch(`/api/projects/rooms/${roomId}/distill`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || `HTTP ${res.status}: Gagal melakukan ekstraksi tugas`)
      }

      const data: DistillResult = await res.json()
      if (data.task) {
        setTitle(data.task.title || title)
        setDescription(data.task.description || description)
        setPriority((data.task.priority as any) || priority)
        setStatus((data.task.status as any) || status)
        if (data.task.tags && data.task.tags.length > 0) {
          setTags(Array.from(new Set([...tags, ...data.task.tags])))
        }
        if (data.task.dod_checklist && data.task.dod_checklist.length > 0) {
          setDodItems(data.task.dod_checklist)
        }
        setDistillStats({
          explicit: data.explicit_dod_count,
          inferred: data.inferred_dod_count,
        })
      }
    } catch (err: any) {
      setError(err.message || 'Gagal mengekstrak draf tugas dari obrolan.')
    } finally {
      setAutoDistilling(false)
    }
  }

  // Submit and create task
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      setError('Judul task wajib diisi.')
      return
    }

    if (dodItems.length === 0) {
      setError('Task wajib memiliki minimal 1 kriteria Definition of Done (DoD).')
      return
    }

    setLoading(true)
    setError(null)

    try {
      const finalProjectIds = selectedProjectIds.length > 0 ? selectedProjectIds : (projectId ? [projectId] : [])
      const primaryProjectId = finalProjectIds[0] || projectId

      if (isEditMode && initialTask) {
        const payload = {
          title: title.trim(),
          description: description.trim(),
          priority,
          status,
          project_ids: finalProjectIds,
          batch_id: batchId.trim() || undefined,
          parent_task_id: parentTaskId.trim() || undefined,
          tags,
          dod_checklist: dodItems,
        }

        const res = await fetch(`/api/tasks/${initialTask.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })

        if (!res.ok) {
          const errData = await res.json().catch(() => ({}))
          throw new Error(errData.error || `HTTP ${res.status}: Gagal memperbarui task`)
        }

        const data = await res.json()
        onTaskUpdated?.(data.task)
        onClose()
      } else {
        const payload = {
          title: title.trim(),
          description: description.trim(),
          priority,
          status,
          project_ids: finalProjectIds,
          room_id: roomId || undefined,
          batch_id: batchId.trim() || undefined,
          parent_task_id: parentTaskId.trim() || undefined,
          tags,
          dod_checklist: dodItems,
        }

        const res = await fetch(`/api/projects/${primaryProjectId}/tasks`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })

        if (!res.ok) {
          const errData = await res.json().catch(() => ({}))
          throw new Error(errData.error || `HTTP ${res.status}: Gagal menyimpan task`)
        }

        const data = await res.json()
        onTaskCreated?.(data.task)
        onClose()
      }
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat menyimpan task.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-background/80 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto animate-in fade-in duration-200">
      <div className="w-full max-w-2xl bg-card text-card-foreground rounded-2xl shadow-2xl border border-border overflow-hidden my-8 animate-in zoom-in-95 duration-200">
        {/* Modal Header */}
        <div className="p-5 border-b border-border flex items-center justify-between bg-muted/30">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-primary/10 border border-primary/20 text-primary flex items-center justify-center">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-foreground tracking-tight flex items-center gap-2">
                {isEditMode ? 'Edit Task & Multi-Project Linking' : 'Distill Discussion to Task'}
                <Badge variant="outline" className="border-primary/30 text-primary text-[10px]">
                  {isEditMode ? 'Edit Task Mode' : 'Sprint Task Draft'}
                </Badge>
              </h2>
              <p className="text-xs text-muted-foreground">
                {isEditMode
                  ? 'Perbarui parameter tugas, acceptance criteria DoD, dan penautan multi-proyek.'
                  : 'Ekstraksi kesepakatan obrolan menjadi item backlog terstruktur dengan acceptance criteria checklist.'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Error Alert */}
        {error && (
          <div className="p-3.5 mx-5 mt-4 bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900 text-red-700 dark:text-red-300 rounded-xl text-xs flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Distill Stats Banner */}
        {distillStats && (
          <div className="p-3 mx-5 mt-4 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300 rounded-xl text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-emerald-600" />
              <span>
                Berhasil menyuling obrolan: <strong>{distillStats.explicit} explicit DoD</strong> +{' '}
                <strong>{distillStats.inferred} inferred DoD</strong> terverifikasi.
              </span>
            </div>
            <Badge className="bg-emerald-600 text-white text-[10px] py-0 px-1.5">Verified</Badge>
          </div>
        )}

        {/* Modal Form */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          {/* Quick Action: Auto-Distill from AI */}
          {roomId && (
            <div className="p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 flex items-center justify-between gap-3">
              <div className="text-xs text-slate-600 dark:text-slate-300">
                <span className="font-semibold block">Gunakan AI Distiller Engine Otomatis</span>
                <span className="text-[11px] text-slate-400">
                  Analisis transkrip obrolan room untuk menghasilkan Title, deskripsi, dan DoD checklist.
                </span>
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleAutoDistill}
                disabled={autoDistilling}
                className="h-8 text-xs font-semibold px-3 border-emerald-400/60 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-50 dark:hover:bg-emerald-950 shrink-0 gap-1.5"
              >
                <Sparkles className={`h-3.5 w-3.5 ${autoDistilling ? 'animate-spin' : ''}`} />
                <span>{autoDistilling ? 'Menganalisis...' : 'Auto-Distill via AI'}</span>
              </Button>
            </div>
          )}

          {/* Title Input */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-wider block">
              Task Title <span className="text-red-500">*</span>
            </label>
            <Input
              type="text"
              placeholder="e.g. Implementasikan SSE Real-Time Streaming listener"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="text-sm font-medium"
              required
            />
          </div>

          {/* Multi-Project Linking Dropdown (S14.4) */}
          <div className="space-y-1.5 relative" ref={projectDropdownRef}>
            <div className="flex items-center justify-between">
              <label className="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-wider block">
                Tautkan ke Proyek (Multi-Project Linking)
              </label>
              <span className="text-[10px] text-slate-400 font-medium">
                Pilih satu atau lebih proyek aktif
              </span>
            </div>

            <div className="relative">
              <button
                type="button"
                onClick={() => setIsProjectDropdownOpen(!isProjectDropdownOpen)}
                className="w-full text-xs py-2 px-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-100 font-medium flex items-center justify-between hover:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition-colors shadow-xs"
              >
                <div className="flex items-center gap-2 truncate">
                  <FolderGit2 className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
                  <span className="truncate">
                    {selectedProjectIds.length === 0
                      ? 'Pilih satu atau lebih proyek aktif...'
                      : selectedProjectIds.length === 1
                      ? activeProjectsList.find((p) => p.id === selectedProjectIds[0])?.name || selectedProjectIds[0]
                      : `${selectedProjectIds.length} Proyek Terpilih (${selectedProjectIds
                          .map((id) => activeProjectsList.find((p) => p.id === id)?.name || id)
                          .join(', ')})`}
                  </span>
                </div>
                <ChevronDown
                  className={cn(
                    'h-3.5 w-3.5 text-slate-400 transition-transform duration-200 shrink-0',
                    isProjectDropdownOpen && 'rotate-180 text-emerald-500'
                  )}
                />
              </button>

              {/* Popover Dropdown with Checkboxes */}
              {isProjectDropdownOpen && (
                <div className="absolute left-0 top-full mt-1.5 z-40 w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-xl p-2 max-h-56 overflow-y-auto space-y-1 animate-in fade-in-50 zoom-in-95">
                  <div className="flex items-center justify-between px-2 py-1 text-[10px] font-bold text-slate-400 uppercase tracking-wider border-b border-slate-100 dark:border-slate-800">
                    <span>Daftar Proyek Aktif</span>
                    <span>{activeProjectsList.length} Proyek</span>
                  </div>

                  {activeProjectsList.length === 0 ? (
                    <div className="p-3 text-center text-xs text-slate-400">
                      Tidak ada proyek aktif ditemukan
                    </div>
                  ) : (
                    activeProjectsList.map((p) => {
                      const isChecked = selectedProjectIds.includes(p.id)
                      return (
                        <label
                          key={p.id}
                          className={cn(
                            'flex items-center gap-2.5 px-2.5 py-2 rounded-lg cursor-pointer text-xs select-none transition-colors',
                            isChecked
                              ? 'bg-emerald-50/80 dark:bg-emerald-950/40 text-emerald-900 dark:text-emerald-200 font-semibold'
                              : 'hover:bg-slate-100 dark:hover:bg-slate-800/80 text-slate-700 dark:text-slate-300'
                          )}
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => toggleProjectSelection(p.id)}
                            className="rounded border-slate-300 text-emerald-600 focus:ring-emerald-500 h-4 w-4"
                          />
                          <span className="truncate flex-1">{p.name}</span>
                          <span className="text-[10px] font-mono text-slate-400">{p.id}</span>
                        </label>
                      )
                    })
                  )}
                </div>
              )}
            </div>

            {/* Selected Projects Badges with Quick Removal */}
            {selectedProjectIds.length > 0 && (
              <div className="flex flex-wrap gap-1.5 pt-1">
                {selectedProjectIds.map((pid) => {
                  const proj = activeProjectsList.find((p) => p.id === pid)
                  return (
                    <Badge
                      key={pid}
                      variant="secondary"
                      className="text-[11px] font-medium bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800 flex items-center gap-1.5 py-0.5 px-2"
                    >
                      <FolderGit2 className="h-3 w-3 text-emerald-500" />
                      <span className="truncate max-w-[150px]">{proj ? proj.name : pid}</span>
                      <button
                        type="button"
                        onClick={() => toggleProjectSelection(pid)}
                        className="text-emerald-600 dark:text-emerald-400 hover:text-emerald-900 dark:hover:text-white font-bold ml-0.5 text-xs"
                        title="Hapus tautan proyek"
                      >
                        ×
                      </button>
                    </Badge>
                  )
                })}
              </div>
            )}
          </div>

          {/* Description Textarea */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-wider block">
              Description / Transcript Context
            </label>
            <textarea
              rows={3}
              placeholder="Rincian scope, konteks percakapan, atau instruksi pengerjaan..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full text-xs font-mono p-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-500 resize-none"
            />
          </div>

          {/* Priority & Status Row */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div className="space-y-1.5">
              <label className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider block">
                Priority
              </label>
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value as any)}
                className="w-full text-xs py-2 px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-100 font-semibold"
              >
                <option value="low">Low</option>
                <option value="normal">Normal</option>
                <option value="high">High</option>
                <option value="urgent">Urgent</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <label className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider block">
                Initial Status
              </label>
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value as any)}
                className="w-full text-xs py-2 px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-slate-100 font-semibold"
              >
                <option value="backlog">Backlog</option>
                <option value="doing">Doing (Triggers Agent)</option>
                <option value="review">Review</option>
                <option value="done">Done</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <label className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider block">
                Batch ID (Optional)
              </label>
              <Input
                type="text"
                placeholder="e.g. batch-s07"
                value={batchId}
                onChange={(e) => setBatchId(e.target.value)}
                className="h-8 text-xs font-mono"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider block">
                Parent Task UUID
              </label>
              <Input
                type="text"
                placeholder="UUID fan-out"
                value={parentTaskId}
                onChange={(e) => setParentTaskId(e.target.value)}
                className="h-8 text-xs font-mono"
              />
            </div>
          </div>

          {/* Tags Chips Section */}
          <div className="space-y-2">
            <label className="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-wider block">
              Tags & Labels ({tags.length})
            </label>
            <div className="flex flex-wrap items-center gap-1.5">
              {tags.map((t) => (
                <span
                  key={t}
                  className="inline-flex items-center gap-1 py-0.5 px-2 rounded-md bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 text-xs font-mono"
                >
                  #{t}
                  <button
                    type="button"
                    onClick={() => handleRemoveTag(t)}
                    className="text-slate-400 hover:text-red-500 ml-0.5"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}

              <div className="flex items-center gap-1">
                <input
                  type="text"
                  placeholder="+ Tambah tag..."
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ',') {
                      e.preventDefault()
                      handleAddTag(tagInput)
                    }
                  }}
                  className="h-6 text-xs px-2 rounded border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 focus:outline-none focus:ring-1 focus:ring-emerald-500 font-mono w-28"
                />
              </div>
            </div>

            {/* Quick Tag Recommendations */}
            <div className="flex items-center gap-1 flex-wrap text-[10px] text-slate-400">
              <span>Saran:</span>
              {['wordpress', 'adsense', 'frontend', 'api', 'backend', 'sse'].map((suggestion) => (
                <button
                  type="button"
                  key={suggestion}
                  onClick={() => handleAddTag(suggestion)}
                  className="hover:text-emerald-500 hover:underline"
                >
                  +{suggestion}
                </button>
              ))}
            </div>
          </div>

          {/* Definition of Done (DoD) Checklist Builder */}
          <div className="space-y-2.5 pt-2 border-t border-slate-100 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <ListTodo className="h-4 w-4 text-emerald-500" />
                <label className="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-wider block">
                  Definition of Done Checklist ({dodItems.length})
                </label>
              </div>
              <Badge
                variant="outline"
                className={`text-[10px] ${
                  dodItems.length >= 3
                    ? 'border-emerald-500 text-emerald-600 dark:text-emerald-400'
                    : 'border-amber-400 text-amber-600 dark:text-amber-400'
                }`}
              >
                {dodItems.length >= 3 ? '✓ Min 3 DoD Terpenuhi' : `${dodItems.length}/3 DoD Disarankan`}
              </Badge>
            </div>

            {/* Existing Checklist Items */}
            <div className="space-y-1.5 max-h-44 overflow-y-auto pr-1">
              {dodItems.map((item, idx) => (
                <div
                  key={idx}
                  className="flex items-center justify-between gap-2 p-2 bg-slate-50 dark:bg-slate-850 rounded-lg border border-slate-200 dark:border-slate-800 text-xs"
                >
                  <label className="flex items-center gap-2 cursor-pointer flex-1 min-w-0">
                    <input
                      type="checkbox"
                      checked={item.completed}
                      onChange={() => handleToggleDoD(idx)}
                      className="rounded text-emerald-600 focus:ring-emerald-500"
                    />
                    <span className={`truncate ${item.completed ? 'line-through text-slate-400' : 'text-slate-800 dark:text-slate-200'}`}>
                      {item.text}
                    </span>
                  </label>
                  <button
                    type="button"
                    onClick={() => handleRemoveDoD(idx)}
                    className="p-1 text-slate-400 hover:text-red-500 rounded"
                    title="Hapus kriteria DoD"
                  >
                    <Trash2 className="h-3 w-3" />
                  </button>
                </div>
              ))}
            </div>

            {/* Add New DoD Input */}
            <div className="flex items-center gap-2">
              <Input
                type="text"
                placeholder="Tambahkan kriteria Definition of Done (DoD)..."
                value={newDodText}
                onChange={(e) => setNewDodText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handleAddDoD()
                  }
                }}
                className="h-8 text-xs"
              />
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleAddDoD}
                disabled={!newDodText.trim()}
                className="h-8 px-2.5 text-xs font-semibold gap-1 shrink-0"
              >
                <Plus className="h-3.5 w-3.5" />
                <span>Tambah</span>
              </Button>
            </div>
          </div>

          {/* Action Footer */}
          <div className="pt-3 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between">
            <span className="text-[11px] text-slate-400">
              {status === 'doing' && (
                <span className="text-amber-600 dark:text-amber-400 font-semibold">
                  ⚡ Memulai status 'doing' akan men-trigger execution runner otomatis.
                </span>
              )}
            </span>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={onClose}
                disabled={loading}
                className="h-9 text-xs"
              >
                Batal
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={loading}
                className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 gap-1.5"
              >
                {loading ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Check className="h-3.5 w-3.5" />
                )}
                <span>{isEditMode ? 'Simpan Perubahan Task' : 'Simpan ke Project Tasks'}</span>
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
