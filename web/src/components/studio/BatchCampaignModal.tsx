import { useState, useEffect, useMemo, FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  X,
  Search,
  CheckSquare,
  Square,
  Layers,
  Sparkles,
  AlertCircle,
  Plus,
  Trash2,
  ListChecks,
  CheckCircle2,
  Terminal,
  FileCode,
  Tag
} from 'lucide-react'
import { BatchCampaignDTO, DoDItem } from './types'

interface ProjectCandidate {
  id: string
  name: string
  tags?: string[]
  repo_url?: string
}

interface BatchCampaignModalProps {
  isOpen: boolean
  onClose: () => void
  onCreated: (campaign: BatchCampaignDTO) => void
  projects?: ProjectCandidate[]
}

const DEFAULT_DOD_ITEMS: DoDItem[] = [
  { text: 'Target repository checkout & workspace isolation verified', completed: false },
  { text: 'Template payload / script executed with exit code 0', completed: false },
  { text: 'Post-execution verification and smoke test passed', completed: false },
]

const TEMPLATE_PRESETS = [
  {
    label: 'AdSense Code Injection',
    script: `<script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-1234567890" crossorigin="anonymous"></script>
<!-- Cuan AdSense Mass Rollout Unit -->
<ins class="adsbygoogle"
     style="display:block"
     data-ad-client="ca-pub-1234567890"
     data-ad-slot="9876543210"
     data-ad-format="auto"
     data-full-width-responsive="true"></ins>
<script>
     (adsbygoogle = window.adsbygoogle || []).push({});
</script>`,
    tags: ['wordpress', 'adsense'],
    defaultTitle: 'AdSense Script Injection Rollout',
    defaultDesc: 'Inject Google AdSense verification code into header & unit tags across all WordPress network sites.',
  },
  {
    label: 'WP-CLI Plugin & Cache Update',
    script: `#!/usr/bin/env bash
set -euo pipefail
echo "[WP-CLI] Updating core plugins & clearing object cache..."
wp plugin update --all --allow-root
wp cache flush --allow-root
wp transient delete --all --allow-root
echo "[WP-CLI] Done successfully."`,
    tags: ['wordpress'],
    defaultTitle: 'WP-CLI Mass Plugin & Cache Maintenance',
    defaultDesc: 'Automated mass maintenance updating plugins and flushing caches across all WordPress sites.',
  },
  {
    label: 'Security Headers Patch',
    script: `#!/usr/bin/env bash
set -euo pipefail
echo "[Security] Applying hardened HTTP headers..."
cat << 'EOF' >> .htaccess
<IfModule mod_headers.c>
  Header always set X-Content-Type-Options "nosniff"
  Header always set X-Frame-Options "SAMEORIGIN"
  Header always set X-XSS-Protection "1; mode=block"
  Header always set Referrer-Policy "strict-origin-when-cross-origin"
</IfModule>
EOF
echo "[Security] Headers applied."`,
    tags: ['security', 'wordpress'],
    defaultTitle: 'Mass HTTP Security Headers Hardening',
    defaultDesc: 'Apply standard security headers across all targeted website configurations.',
  },
]

export function BatchCampaignModal({
  isOpen,
  onClose,
  onCreated,
  projects: propProjects,
}: BatchCampaignModalProps) {
  // Form fields
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [scriptPayload, setScriptPayload] = useState('')
  const [dodItems, setDodItems] = useState<DoDItem[]>(DEFAULT_DOD_ITEMS)
  const [newDodText, setNewDodText] = useState('')

  // Multi-Project Selector states
  const [candidates, setCandidates] = useState<ProjectCandidate[]>([])
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [activeTagFilter, setActiveTagFilter] = useState<string>('all')

  // Status & Validation
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Fetch candidates if not provided
  useEffect(() => {
    if (!isOpen) return

    if (propProjects && propProjects.length > 0) {
      setCandidates(propProjects)
      // Auto-select all by default
      setSelectedIds(propProjects.map((p) => p.id))
      return
    }

    const loadProjects = async () => {
      try {
        const [projRes, depRes] = await Promise.allSettled([
          fetch('/api/v1/projects'),
          fetch('/api/deployments'),
        ])

        const combined: ProjectCandidate[] = []
        const seen = new Set<string>()

        if (projRes.status === 'fulfilled' && projRes.value.ok) {
          const data = await projRes.value.json()
          const pList = data.projects || []
          for (const p of pList) {
            if (!seen.has(p.id)) {
              seen.add(p.id)
              const inferTags: string[] = []
              const lowerName = (p.name || '').toLowerCase()
              if (lowerName.includes('wordpress') || lowerName.includes('wp')) inferTags.push('wordpress')
              if (lowerName.includes('adsense') || lowerName.includes('ad')) inferTags.push('adsense')
              if (lowerName.includes('agency')) inferTags.push('agency')
              if (lowerName.includes('blog')) inferTags.push('blog')
              if (inferTags.length === 0) inferTags.push('general')

              combined.push({
                id: p.id,
                name: p.name,
                repo_url: p.repo_url,
                tags: inferTags,
              })
            }
          }
        }

        if (depRes.status === 'fulfilled' && depRes.value.ok) {
          const data = await depRes.value.json()
          const dList = data.deployments || []
          for (const d of dList) {
            if (!seen.has(d.id)) {
              seen.add(d.id)
              const inferTags: string[] = []
              const lowerName = (d.project_name || '').toLowerCase()
              if (lowerName.includes('wordpress') || lowerName.includes('wp') || d.runtime === 'php') {
                inferTags.push('wordpress')
              }
              if (lowerName.includes('adsense') || lowerName.includes('ad')) inferTags.push('adsense')
              if (inferTags.length === 0) inferTags.push(d.runtime || 'web')

              combined.push({
                id: d.id,
                name: d.project_name,
                tags: inferTags,
              })
            }
          }
        }

        setCandidates(combined)
        setSelectedIds(combined.map((c) => c.id))
      } catch (err) {
        console.error('Failed to load project candidates:', err)
      }
    }

    loadProjects()
  }, [isOpen, propProjects])

  // Extract unique tags from candidates
  const allAvailableTags = useMemo(() => {
    const tagSet = new Set<string>()
    tagSet.add('wordpress')
    tagSet.add('adsense')
    candidates.forEach((c) => {
      c.tags?.forEach((t) => tagSet.add(t.toLowerCase()))
    })
    return Array.from(tagSet)
  }, [candidates])

  // Filter candidates based on search & active tag
  const filteredCandidates = useMemo(() => {
    return candidates.filter((c) => {
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchName = c.name.toLowerCase().includes(q)
        const matchId = c.id.toLowerCase().includes(q)
        if (!matchName && !matchId) return false
      }

      if (activeTagFilter !== 'all') {
        const tag = activeTagFilter.toLowerCase()
        const hasTag = c.tags?.some((t) => t.toLowerCase() === tag)
        const nameHasTag = c.name.toLowerCase().includes(tag)
        if (!hasTag && !nameHasTag) return false
      }

      return true
    })
  }, [candidates, searchQuery, activeTagFilter])

  // Selection handlers
  const handleToggleProject = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
    )
  }

  const handleSelectAllFiltered = () => {
    const idsToAdd = filteredCandidates.map((c) => c.id)
    setSelectedIds((prev) => Array.from(new Set([...prev, ...idsToAdd])))
  }

  const handleDeselectAllFiltered = () => {
    const idsToRemove = new Set(filteredCandidates.map((c) => c.id))
    setSelectedIds((prev) => prev.filter((id) => !idsToRemove.has(id)))
  }

  const handleSelectByTag = (tag: string) => {
    const matchingIds = candidates
      .filter((c) => c.tags?.includes(tag) || c.name.toLowerCase().includes(tag))
      .map((c) => c.id)
    setSelectedIds(matchingIds)
    setActiveTagFilter(tag)
  }

  // DoD Checklist handlers
  const handleAddDodItem = () => {
    const text = newDodText.trim()
    if (!text) return
    setDodItems((prev) => [...prev, { text, completed: false }])
    setNewDodText('')
  }

  const handleRemoveDodItem = (index: number) => {
    if (dodItems.length <= 3) {
      setError('Jaminan DoD minimal: Harus memiliki minimal 3 kriteria penerimaan terverifikasi.')
      return
    }
    setError(null)
    setDodItems((prev) => prev.filter((_, i) => i !== index))
  }

  // Apply Preset Template
  const handleApplyPreset = (preset: typeof TEMPLATE_PRESETS[0]) => {
    if (!title) setTitle(preset.defaultTitle)
    if (!description) setDescription(preset.defaultDesc)
    setScriptPayload(preset.script)
    if (preset.tags && preset.tags.length > 0) {
      handleSelectByTag(preset.tags[0])
    }
  }

  // Submit Handler
  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validation
    if (!title.trim()) {
      setError('Judul kampanye (Campaign Title) wajib diisi.')
      return
    }

    if (selectedIds.length === 0) {
      setError('Pilih minimal 1 proyek target untuk kampanye batch.')
      return
    }

    if (!scriptPayload.trim()) {
      setError('Script / Command Template wajib diisi.')
      return
    }

    if (dodItems.length < 3) {
      setError(`DoD Checklist harus memiliki minimal 3 butir (saat ini: ${dodItems.length} butir).`)
      return
    }

    setLoading(true)

    try {
      const targetTags = activeTagFilter !== 'all' ? [activeTagFilter] : []
      const payload = {
        title: title.trim(),
        description: description.trim(),
        target_criteria: {
          project_ids: selectedIds,
          tags: targetTags,
          all: selectedIds.length === candidates.length,
        },
        template_payload: {
          title_template: `[Campaign] ${title.trim()} - {{project_name}}`,
          instructions: description.trim(),
          script: scriptPayload.trim(),
          tags: targetTags,
          dod_checklist: dodItems.map((item) => ({
            text: item.text,
            completed: false,
          })),
        },
        status: 'draft',
      }

      const res = await fetch('/api/campaigns', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}: Gagal membuat kampanye batch`)
      }

      onCreated(data.campaign)
      onClose()
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan sistem saat membuat kampanye.')
    } finally {
      setLoading(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-sm overflow-y-auto animate-in fade-in duration-200">
      <div className="relative w-full max-w-4xl my-8 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 text-white flex items-center justify-center shadow-md shadow-emerald-500/20">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                New Batch Campaign Studio
              </h2>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Multi-Project Selector, Script Template & DoD Verification Builder
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Modal Body */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Validasi Gagal</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {/* Section 1: Campaign Details */}
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                Campaign Title <span className="text-red-500">*</span>
              </label>
              <Input
                type="text"
                placeholder="Contoh: Mass AdSense Code Insertion & Verification"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="h-10 text-sm"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                Campaign Description & Goals
              </label>
              <textarea
                rows={2}
                placeholder="Jelaskan tujuan dan cakupan rollout kampanye batch ini..."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder:text-slate-400 focus:outline-hidden focus:ring-2 focus:ring-emerald-500"
              />
            </div>
          </div>

          {/* Section 2: Multi-Project Selector */}
          <div className="space-y-3 p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/50">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <div className="flex items-center gap-2">
                  <Layers className="h-4 w-4 text-emerald-500" />
                  <span className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                    Multi-Project Selector
                  </span>
                  <Badge variant="outline" className="border-emerald-500/50 text-emerald-600 dark:text-emerald-400 text-[10px] font-mono">
                    {selectedIds.length} projects selected
                  </Badge>
                </div>
                <p className="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                  Pilih situs/proyek target menggunakan pencarian, filter chips tag, atau seleksi manual.
                </p>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleSelectAllFiltered}
                  className="h-7 text-[11px] px-2.5"
                >
                  <CheckSquare className="h-3 w-3 mr-1 text-emerald-500" /> Select All
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleDeselectAllFiltered}
                  className="h-7 text-[11px] px-2.5"
                >
                  <Square className="h-3 w-3 mr-1 text-slate-400" /> Deselect All
                </Button>
              </div>
            </div>

            {/* Filter Chips & Search Bar */}
            <div className="flex flex-col sm:flex-row gap-2 pt-1">
              <div className="relative flex-1">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-400" />
                <Input
                  type="text"
                  placeholder="Cari nama proyek atau repo..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="h-8 pl-8 text-xs bg-white dark:bg-slate-900"
                />
              </div>

              {/* Tag Chips */}
              <div className="flex flex-wrap items-center gap-1.5 overflow-x-auto py-0.5">
                <button
                  type="button"
                  onClick={() => setActiveTagFilter('all')}
                  className={`text-[10px] px-2.5 py-1 rounded-full font-medium transition-colors ${
                    activeTagFilter === 'all'
                      ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900'
                      : 'bg-slate-200/80 dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:bg-slate-300'
                  }`}
                >
                  All ({candidates.length})
                </button>

                {allAvailableTags.map((t) => {
                  const matchCount = candidates.filter(
                    (c) => c.tags?.includes(t) || c.name.toLowerCase().includes(t)
                  ).length
                  const isActive = activeTagFilter === t
                  return (
                    <button
                      key={t}
                      type="button"
                      onClick={() => handleSelectByTag(t)}
                      className={`text-[10px] px-2.5 py-1 rounded-full font-medium transition-colors flex items-center gap-1 ${
                        isActive
                          ? 'bg-emerald-600 text-white'
                          : 'bg-emerald-100 dark:bg-emerald-950/60 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-200'
                      }`}
                    >
                      <Tag className="h-2.5 w-2.5" />
                      {t} ({matchCount})
                    </button>
                  )
                })}
              </div>
            </div>

            {/* Project Cards Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5 max-h-48 overflow-y-auto p-1 border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900">
              {filteredCandidates.length === 0 ? (
                <div className="col-span-full py-6 text-center text-xs text-slate-400">
                  Tidak ada proyek yang cocok dengan filter atau kata kunci.
                </div>
              ) : (
                filteredCandidates.map((p) => {
                  const isSelected = selectedIds.includes(p.id)
                  return (
                    <div
                      key={p.id}
                      onClick={() => handleToggleProject(p.id)}
                      className={`cursor-pointer p-2.5 rounded-lg border text-left transition-all flex items-start gap-2.5 select-none ${
                        isSelected
                          ? 'border-emerald-500 bg-emerald-50/40 dark:bg-emerald-950/20 text-slate-900 dark:text-white'
                          : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 text-slate-600 dark:text-slate-400'
                      }`}
                    >
                      <div className="mt-0.5 shrink-0">
                        {isSelected ? (
                          <CheckSquare className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                        ) : (
                          <Square className="h-4 w-4 text-slate-300 dark:text-slate-600" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-xs font-semibold truncate leading-tight">{p.name}</p>
                        <p className="text-[10px] text-slate-400 font-mono truncate">{p.id.slice(0, 14)}...</p>
                        <div className="flex flex-wrap gap-1 mt-1">
                          {p.tags?.slice(0, 2).map((tg) => (
                            <span
                              key={tg}
                              className="text-[9px] px-1.5 py-0.2 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 font-mono"
                            >
                              {tg}
                            </span>
                          ))}
                        </div>
                      </div>
                    </div>
                  )
                })
              )}
            </div>
            <div className="text-[11px] text-slate-500 text-right">
              Terpilih: <strong className="text-emerald-600 dark:text-emerald-400">{selectedIds.length}</strong> dari {candidates.length} proyek terdaftar
            </div>
          </div>

          {/* Section 3: Script / Command Template Editor */}
          <div className="space-y-3 p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/50">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <div className="flex items-center gap-2">
                  <Terminal className="h-4 w-4 text-emerald-500" />
                  <span className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                    Script / Command Template Editor
                  </span>
                </div>
                <p className="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                  Payload skrip atau perintah shell / WP-CLI yang dieksekusi terisolasi di setiap proyek target.
                </p>
              </div>

              {/* Template Presets */}
              <div className="flex flex-wrap gap-1.5">
                {TEMPLATE_PRESETS.map((preset) => (
                  <Button
                    key={preset.label}
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => handleApplyPreset(preset)}
                    className="h-6 text-[10px] px-2 bg-white dark:bg-slate-900 border-slate-300 dark:border-slate-700"
                  >
                    <FileCode className="h-3 w-3 mr-1 text-emerald-500" />
                    {preset.label}
                  </Button>
                ))}
              </div>
            </div>

            <textarea
              rows={6}
              placeholder="Masukkan skrip eksekusi shell, snippet HTML/JS, atau perintah CLI di sini..."
              value={scriptPayload}
              onChange={(e) => setScriptPayload(e.target.value)}
              className="w-full font-mono text-xs rounded-md border border-slate-300 dark:border-slate-700 bg-slate-950 text-emerald-400 px-3 py-2.5 focus:outline-hidden focus:ring-2 focus:ring-emerald-500 placeholder:text-slate-600"
              required
            />
          </div>

          {/* Section 4: Definition of Done (DoD) Checklist Builder */}
          <div className="space-y-3 p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/50">
            <div className="flex items-center justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <ListChecks className="h-4 w-4 text-emerald-500" />
                  <span className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                    Definition of Done (DoD) Checklist Builder
                  </span>
                  <Badge
                    variant="outline"
                    className={`text-[10px] font-mono ${
                      dodItems.length >= 3
                        ? 'border-emerald-500 text-emerald-600 dark:text-emerald-400'
                        : 'border-red-500 text-red-500'
                    }`}
                  >
                    {dodItems.length} items (Min. 3 Guaranteed)
                  </Badge>
                </div>
                <p className="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                  Kriteria penerimaan terverifikasi yang harus dipenuhi oleh setiap subtask proyek anak.
                </p>
              </div>
            </div>

            {/* Checklist Items */}
            <div className="space-y-2">
              {dodItems.map((item, idx) => (
                <div
                  key={idx}
                  className="flex items-center gap-2 p-2 rounded-lg bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-xs"
                >
                  <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0" />
                  <span className="font-mono text-slate-400 text-[10px] w-5">#{idx + 1}</span>
                  <input
                    type="text"
                    value={item.text}
                    onChange={(e) => {
                      const newText = e.target.value
                      setDodItems((prev) =>
                        prev.map((it, i) => (i === idx ? { ...it, text: newText } : it))
                      )
                    }}
                    className="flex-1 bg-transparent border-none text-xs text-slate-900 dark:text-slate-100 focus:outline-hidden"
                  />
                  <button
                    type="button"
                    onClick={() => handleRemoveDodItem(idx)}
                    disabled={dodItems.length <= 3}
                    title={dodItems.length <= 3 ? 'Minimal 3 butir DoD wajib dipertahankan' : 'Hapus butir'}
                    className={`p-1 rounded text-slate-400 hover:text-red-500 transition-colors ${
                      dodItems.length <= 3 ? 'opacity-30 cursor-not-allowed' : ''
                    }`}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>

            {/* Add DoD Item Input */}
            <div className="flex gap-2 pt-1">
              <Input
                type="text"
                placeholder="Tambah kriteria acceptance DoD baru..."
                value={newDodText}
                onChange={(e) => setNewDodText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handleAddDodItem()
                  }
                }}
                className="h-8 text-xs bg-white dark:bg-slate-900"
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleAddDodItem}
                className="h-8 text-xs px-3 shrink-0"
              >
                <Plus className="h-3.5 w-3.5 mr-1" /> Tambah Butir
              </Button>
            </div>
          </div>
        </form>

        {/* Modal Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 rounded-b-2xl shrink-0">
          <div className="text-xs text-slate-500">
            Target: <strong className="text-slate-900 dark:text-white">{selectedIds.length} Proyek</strong> | DoD: <strong className="text-slate-900 dark:text-white">{dodItems.length} Butir</strong>
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onClose}
              disabled={loading}
              className="h-9 px-4 text-xs"
            >
              Batal
            </Button>
            <Button
              type="button"
              onClick={handleSubmit}
              disabled={loading || selectedIds.length === 0 || dodItems.length < 3}
              className="h-9 px-5 text-xs bg-emerald-600 hover:bg-emerald-500 text-white font-semibold shadow-md shadow-emerald-600/20"
            >
              {loading ? 'Menyimpan...' : 'Create Batch Campaign'}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
