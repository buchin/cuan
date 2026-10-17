import { useState, useEffect } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Sparkles,
  BookOpen,
  Video,
  ShoppingBag,
  Percent,
  CheckCircle2,
  X,
  ArrowRight,
  Bot,
  MessageSquare,
  ListTodo,
  Layers,
  FolderGit2
} from 'lucide-react'
import {
  CreatorTemplate,
  InstantiateCreatorProjectResponse,
  CreatorTaskTemplate
} from './types'

interface CreatorWizardModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: (result: InstantiateCreatorProjectResponse) => void
}

const FALLBACK_TEMPLATES: CreatorTemplate[] = [
  {
    id: 'blog',
    name: 'Blog Publisher',
    description: 'Automated content creation and SEO pipeline for niche bloggers and AdSense publishers.',
    category: 'content',
    icon: 'book-open',
    room: {
      title: 'SEO Research',
      topic: 'SEO keyword research, content planning, and pillar article drafts',
    },
    agent: {
      name: 'SEO Specialist',
      role: 'Search Engine Optimization Specialist & Content Strategist',
      avatar: 'seo-bot',
      system_prompt: 'Anda adalah SEO Specialist & Content Strategist.',
      model_id: 'gemini-1.5-pro',
      provider: 'google',
    },
    tasks: [
      {
        title: 'Keyword Research & Topic Clusters',
        description: 'Lakukan riset keyword niche, cari search intent dan volume, rancang 5 pillar cluster.',
        priority: 'high',
        dod_checklist: [
          { text: 'Daftar 10 keyword primer teridentifikasi dengan search intent', completed: false },
          { text: 'Rancangan 5 topic cluster dengan arsitektur internal linking', completed: false },
          { text: 'Analisis gap konten kompetitor', completed: false },
        ],
        tags: ['seo', 'research', 'keyword'],
        execution_status: 'idle',
      },
      {
        title: 'Pillar Article Drafting',
        description: 'Draft artikel pilar 2.000+ kata lengkap dengan H2, H3, intro hook, dan call-to-action.',
        priority: 'urgent',
        dod_checklist: [
          { text: 'Draft artikel minimal 2.000 kata berstruktur rapi', completed: false },
          { text: 'Heading H1, H2, H3 teroptimasi target keyword', completed: false },
          { text: 'Meta title dan meta description siap publish', completed: false },
        ],
        tags: ['content', 'drafting', 'blog'],
        execution_status: 'idle',
      },
      {
        title: 'On-Page SEO & Content Optimization',
        description: 'Review keterbacaan artikel, optimasi keyword density, siapkan alt text gambar, dan FAQ schema.',
        priority: 'normal',
        dod_checklist: [
          { text: 'Skor keterbacaan Flesch di atas 60', completed: false },
          { text: 'Daftar 3 alt text gambar relevan', completed: false },
          { text: 'JSON-LD FAQ Schema markup tergenerate', completed: false },
        ],
        tags: ['seo', 'optimization', 'schema'],
        execution_status: 'idle',
      },
    ],
  },
  {
    id: 'youtube',
    name: 'YouTube Channel',
    description: 'End-to-end video production workflow from ideation to 10-minute scripts and SEO tagging.',
    category: 'video',
    icon: 'video',
    room: {
      title: 'Channel Production',
      topic: 'YouTube video production, hook ideation, full scriptwriting, and metadata',
    },
    agent: {
      name: 'YouTube Scriptwriter',
      role: 'YouTube Video Producer & Viral Scriptwriter',
      avatar: 'video-bot',
      system_prompt: 'Anda adalah produser YouTube dan scriptwriter handal.',
      model_id: 'gemini-1.5-pro',
      provider: 'google',
    },
    tasks: [
      {
        title: 'Video Ideation & Trend Research',
        description: 'Riset 3 ide video viral dengan CTR tinggi berbasis topik trending dan analisis penonton.',
        priority: 'high',
        dod_checklist: [
          { text: '3 konsep video dengan target audience jelas', completed: false },
          { text: 'Bukti referensi video viral sejenis dengan retensi tinggi', completed: false },
        ],
        tags: ['youtube', 'ideation', 'trends'],
        execution_status: 'idle',
      },
      {
        title: 'Video Hook & Title Generation',
        description: 'Buat 5 variasi judul dengan CTR tinggi dan rancang 30-detik hook pertama untuk retensi.',
        priority: 'urgent',
        dod_checklist: [
          { text: '5 opsi judul click-worthy tanpa clickbait murahan', completed: false },
          { text: 'Skrip hook 30 detik pertama pencegah drop-off', completed: false },
        ],
        tags: ['youtube', 'hook', 'title'],
        execution_status: 'idle',
      },
      {
        title: 'Full 10-Minute Video Scriptwriting',
        description: 'Tulis skrip video berdurasi 10 menit lengkap dengan narasi suara, visual cues, dan transisi b-roll.',
        priority: 'urgent',
        dod_checklist: [
          { text: 'Skrip lengkap 1.200-1.500 kata dengan format narasi & visual cues', completed: false },
          { text: 'Penyisipan mid-roll re-hook dan call-to-action subscribe/like', completed: false },
        ],
        tags: ['youtube', 'script', 'production'],
        execution_status: 'idle',
      },
      {
        title: 'YouTube Description & Tagging',
        description: 'Buat deskripsi video SEO-friendly lengkap dengan timestamp bab, link afiliasi, dan hashtag.',
        priority: 'normal',
        dod_checklist: [
          { text: 'Deskripsi video 300 kata kaya keyword', completed: false },
          { text: 'Struktur chapters/timestamps 00:00', completed: false },
          { text: '15 YouTube tags teroptimasi pencarian', completed: false },
        ],
        tags: ['youtube', 'seo', 'metadata'],
        execution_status: 'idle',
      },
    ],
  },
  {
    id: 'olshop',
    name: 'Online Shop / Digital Product',
    description: 'High-conversion sales copy, 3-part email nurture sequences, and customer FAQ generation.',
    category: 'ecommerce',
    icon: 'shopping-bag',
    room: {
      title: 'Sales Funnel',
      topic: 'Direct-response copywriting, launch funnel, email nurture, and objection handling',
    },
    agent: {
      name: 'Direct-Response Copywriter',
      role: 'Direct-Response Copywriter & E-Commerce Funnel Strategist',
      avatar: 'copy-bot',
      system_prompt: 'Anda adalah Direct-Response Copywriter.',
      model_id: 'gemini-1.5-pro',
      provider: 'google',
    },
    tasks: [
      {
        title: 'Sales Page Copywriting',
        description: 'Tulis landing page copy dengan formula PAS (Problem-Agitate-Solution) berkonversi tinggi.',
        priority: 'urgent',
        dod_checklist: [
          { text: 'Headline & sub-headline kuat dengan value proposition jelas', completed: false },
          { text: 'Penyusunan problem-agitation yang menyentuh emosi pembeli', completed: false },
          { text: 'Penjelasan manfaat produk & call-to-action tombol beli', completed: false },
        ],
        tags: ['copywriting', 'sales-page', 'funnel'],
        execution_status: 'idle',
      },
      {
        title: '3-Part Email Nurture Sequence',
        description: 'Rancang 3 urutan email otomatis (Welcome, Value Story, Hard Pitch) untuk subscriber baru.',
        priority: 'high',
        dod_checklist: [
          { text: 'Email 1: Welcome & delivery lead magnet', completed: false },
          { text: 'Email 2: Value story & mengatasi keraguan calon pembeli', completed: false },
          { text: 'Email 3: Hard pitch dengan penawaran berbatas waktu', completed: false },
        ],
        tags: ['email', 'nurture', 'sales'],
        execution_status: 'idle',
      },
      {
        title: 'FAQ & Objection Handling Matrix',
        description: 'Identifikasi 7 keraguan terbesar pembeli dan buat jawaban meyakinkan untuk meminimalkan refund.',
        priority: 'normal',
        dod_checklist: [
          { text: '7 pertanyaan umum seputar harga, garansi, dan teknis terjawab', completed: false },
          { text: 'Bahasa persuasif yang meredakan keraguan calon konsumen', completed: false },
        ],
        tags: ['faq', 'copywriting', 'support'],
        execution_status: 'idle',
      },
    ],
  },
  {
    id: 'affiliate',
    name: 'Affiliate Site',
    description: 'Product comparison matrices, deep feature breakdowns, and conversion-optimized reviews.',
    category: 'affiliate',
    icon: 'percent',
    room: {
      title: 'Niche Affiliate',
      topic: 'Affiliate product reviews, comparison tables, and buyer intent conversion',
    },
    agent: {
      name: 'Affiliate Reviewer',
      role: 'Affiliate Marketing Specialist & In-Depth Reviewer',
      avatar: 'affiliate-bot',
      system_prompt: 'Anda adalah Affiliate Marketing Reviewer.',
      model_id: 'gemini-1.5-pro',
      provider: 'google',
    },
    tasks: [
      {
        title: 'Product Feature Matrix & Comparison',
        description: 'Susun tabel perbandingan fitur, harga, kelebihan dan kekurangan 3 produk kompetitor niche.',
        priority: 'high',
        dod_checklist: [
          { text: 'Matriks komparasi 3 produk dengan 5 parameter utama', completed: false },
          { text: 'Rekomendasi spesifik berdasarkan tipe pengguna (pemula vs pro)', completed: false },
        ],
        tags: ['affiliate', 'comparison', 'matrix'],
        execution_status: 'idle',
      },
      {
        title: 'In-Depth Product Review Article',
        description: 'Tulis ulasan mendalam pengalaman penggunaan produk dengan rating objektif dan link afiliasi terarah.',
        priority: 'urgent',
        dod_checklist: [
          { text: 'Review 1.500 kata dengan format Verdict, Pros/Cons, dan Pricing', completed: false },
          { text: 'Callout box khusus penempatan link/kupon diskon affiliate', completed: false },
        ],
        tags: ['affiliate', 'review', 'content'],
        execution_status: 'idle',
      },
      {
        title: 'Affiliate Call-to-Action Optimization',
        description: 'Rancang 3 gaya tombol call-to-action persuasif dengan trigger kelangkaan (scarcity).',
        priority: 'normal',
        dod_checklist: [
          { text: '3 variasi copy tombol CTA berorientasi klik tinggi', completed: false },
          { text: 'Penempatan tombol di 3 titik strategis (atas, tengah, bawah)', completed: false },
        ],
        tags: ['affiliate', 'cta', 'conversion'],
        execution_status: 'idle',
      },
    ],
  },
]

export function CreatorWizardModal({ isOpen, onClose, onSuccess }: CreatorWizardModalProps) {
  const [templates, setTemplates] = useState<CreatorTemplate[]>(FALLBACK_TEMPLATES)
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>('blog')
  const [projectName, setProjectName] = useState<string>('Blog Publisher Niche')
  const [repoUrl, setRepoUrl] = useState<string>('')
  const [defaultBranch, setDefaultBranch] = useState<string>('main')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Fetch templates from API
  useEffect(() => {
    if (!isOpen) return

    const fetchTemplates = async () => {
      try {
        const res = await fetch('/api/v1/creator/templates')
        if (res.ok) {
          const data = await res.json()
          if (data.templates && Array.isArray(data.templates) && data.templates.length > 0) {
            setTemplates(data.templates)
          }
        }
      } catch {
        // Fallback to default
      }
    }

    fetchTemplates()
  }, [isOpen])

  // Auto-update project name when template changes if user hasn't heavily customized it
  const handleSelectTemplate = (tmpl: CreatorTemplate) => {
    setSelectedTemplateId(tmpl.id)
    if (tmpl.id === 'blog') setProjectName('Blog Publisher Niche')
    else if (tmpl.id === 'youtube') setProjectName('YouTube Production Studio')
    else if (tmpl.id === 'olshop') setProjectName('Digital Product Sales Funnel')
    else if (tmpl.id === 'affiliate') setProjectName('Niche Affiliate Review Site')
    else setProjectName(`${tmpl.name} Project`)
  }

  const selectedTemplate = templates.find((t) => t.id === selectedTemplateId) || templates[0]

  const getTemplateIcon = (id: string) => {
    switch (id) {
      case 'blog':
        return <BookOpen className="h-6 w-6 text-emerald-500" />
      case 'youtube':
        return <Video className="h-6 w-6 text-red-500" />
      case 'olshop':
        return <ShoppingBag className="h-6 w-6 text-blue-500" />
      case 'affiliate':
        return <Percent className="h-6 w-6 text-amber-500" />
      default:
        return <Sparkles className="h-6 w-6 text-purple-500" />
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (!projectName.trim()) {
      setError('Nama project creator wajib diisi.')
      return
    }

    setLoading(true)
    try {
      const payload = {
        template_id: selectedTemplateId,
        project_name: projectName.trim(),
        repo_url: repoUrl.trim() || undefined,
        default_branch: defaultBranch.trim() || undefined,
        metadata: {
          creator_category: selectedTemplate.category,
          source: 'creator_wizard',
          created_via: 'studio_modal',
        },
      }

      const res = await fetch('/api/v1/creator/projects/instantiate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || data.message || `HTTP ${res.status}: Gagal membuat creator project`)
      }

      onSuccess(data)
      onClose()
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan saat menginstansiasi project creator.')
    } finally {
      setLoading(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl w-full max-w-4xl shadow-2xl overflow-hidden flex flex-col max-h-[92vh]">
        {/* Header */}
        <div className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-gradient-to-r from-emerald-500/10 via-teal-500/5 to-slate-50 dark:to-slate-900">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-2xl bg-emerald-500 text-slate-950 font-bold flex items-center justify-center shadow-lg shadow-emerald-500/20">
              <Sparkles className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-lg font-black text-slate-900 dark:text-white tracking-tight">
                  Creator Project Wizard
                </h2>
                <Badge className="bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 text-[10px] font-mono py-0 px-2">
                  4 Blueprints
                </Badge>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Pilih blueprint alur kerja kreator dan generate project lengkap dengan rooms, tasks, dan AI Agent persona.
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
          {error && (
            <Alert variant="destructive" className="bg-red-500/10 border-red-500/30 text-red-600 dark:text-red-400">
              <AlertTitle className="text-xs font-bold">Gagal Membuat Project</AlertTitle>
              <AlertDescription className="text-xs">{error}</AlertDescription>
            </Alert>
          )}

          {/* 1. Template Catalog Selection */}
          <div className="space-y-3">
            <label className="text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider flex items-center gap-1.5">
              <Layers className="h-4 w-4 text-emerald-500" />
              <span>1. Pilih Template Blueprint Kreator</span>
            </label>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
              {templates.map((tmpl) => {
                const isSelected = tmpl.id === selectedTemplateId
                return (
                  <div
                    key={tmpl.id}
                    onClick={() => handleSelectTemplate(tmpl)}
                    className={`cursor-pointer rounded-2xl p-4 border transition-all relative flex flex-col justify-between text-left ${
                      isSelected
                        ? 'border-emerald-500 bg-emerald-500/5 dark:bg-emerald-500/10 shadow-md ring-2 ring-emerald-500/20'
                        : 'border-slate-200 dark:border-slate-800 hover:border-slate-300 dark:hover:border-slate-700 bg-white dark:bg-slate-950/60'
                    }`}
                  >
                    {isSelected && (
                      <div className="absolute top-3 right-3 text-emerald-500">
                        <CheckCircle2 className="h-5 w-5 fill-emerald-500 text-white dark:text-slate-900" />
                      </div>
                    )}

                    <div className="space-y-2">
                      <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
                        {getTemplateIcon(tmpl.id)}
                      </div>
                      <div>
                        <h4 className="text-sm font-bold text-slate-900 dark:text-white">
                          {tmpl.name}
                        </h4>
                        <span className="text-[10px] font-mono uppercase text-slate-400 block mt-0.5">
                          {tmpl.category}
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-500 dark:text-slate-400 line-clamp-2 leading-relaxed">
                        {tmpl.description}
                      </p>
                    </div>

                    <div className="pt-3 mt-3 border-t border-slate-100 dark:border-slate-800/80 flex items-center justify-between text-[10px] text-slate-400">
                      <span className="flex items-center gap-1 font-medium">
                        <ListTodo className="h-3 w-3 text-emerald-500" />
                        {tmpl.tasks?.length || 3} Tasks
                      </span>
                      <span className="font-medium text-emerald-600 dark:text-emerald-400">
                        {tmpl.agent?.name?.split(' ')[0] || 'AI Agent'}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          {/* 2. Blueprint Breakdown Details */}
          {selectedTemplate && (
            <div className="p-4 rounded-2xl bg-slate-50 dark:bg-slate-950/70 border border-slate-200 dark:border-slate-800 space-y-4">
              <div className="flex items-center justify-between flex-wrap gap-2">
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-emerald-500/10 text-emerald-500">
                    <Bot className="h-4 w-4" />
                  </div>
                  <span className="text-xs font-bold text-slate-800 dark:text-slate-200">
                    AI Persona: <strong className="text-emerald-600 dark:text-emerald-400">{selectedTemplate.agent?.name}</strong>
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-blue-500/10 text-blue-500">
                    <MessageSquare className="h-4 w-4" />
                  </div>
                  <span className="text-xs font-bold text-slate-800 dark:text-slate-200">
                    Room: <span className="font-mono text-blue-600 dark:text-blue-400">#{selectedTemplate.room?.title}</span>
                  </span>
                </div>
              </div>

              {/* Tasks Pipeline Preview */}
              <div className="space-y-1.5">
                <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider block">
                  Pipeline Tasks yang Siap Dibuat ({selectedTemplate.tasks?.length || 0}):
                </span>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {selectedTemplate.tasks?.map((task: CreatorTaskTemplate, idx: number) => (
                    <div
                      key={idx}
                      className="p-2.5 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 flex items-start gap-2 text-xs"
                    >
                      <span className="h-5 w-5 rounded-full bg-slate-100 dark:bg-slate-800 text-[10px] font-mono flex items-center justify-center text-slate-500 shrink-0 font-bold">
                        {idx + 1}
                      </span>
                      <div className="min-w-0 flex-1">
                        <span className="font-bold text-slate-800 dark:text-slate-200 block truncate">
                          {task.title}
                        </span>
                        <span className="text-[10px] text-slate-400">
                          {task.dod_checklist?.length || 0} kriteria Definition of Done (DoD)
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* 3. Project Configuration */}
          <div className="space-y-4">
            <label className="text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider flex items-center gap-1.5">
              <span>2. Konfigurasi Proyek</span>
            </label>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  Nama Proyek Creator <span className="text-red-500">*</span>
                </label>
                <Input
                  type="text"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  placeholder="Contoh: My Tech Blog 2026"
                  required
                  className="text-xs h-10 rounded-xl bg-white dark:bg-slate-950 border-slate-200 dark:border-slate-800"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center justify-between">
                  <span>Git Repository URL (Opsional)</span>
                  <span className="text-[10px] text-slate-400 font-normal">GitHub / GitLab</span>
                </label>
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <Input
                      type="url"
                      value={repoUrl}
                      onChange={(e) => setRepoUrl(e.target.value)}
                      placeholder="https://github.com/username/project.git"
                      className="text-xs h-10 rounded-xl bg-white dark:bg-slate-950 border-slate-200 dark:border-slate-800 pl-8"
                    />
                    <FolderGit2 className="h-4 w-4 text-slate-400 absolute left-2.5 top-3" />
                  </div>
                  <Input
                    type="text"
                    value={defaultBranch}
                    onChange={(e) => setDefaultBranch(e.target.value)}
                    placeholder="main"
                    title="Default Branch"
                    className="w-24 text-xs h-10 rounded-xl bg-white dark:bg-slate-950 border-slate-200 dark:border-slate-800 text-center font-mono"
                  />
                </div>
              </div>
            </div>
          </div>
        </form>

        {/* Footer Actions */}
        <div className="px-6 py-4 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 flex items-center justify-between">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={loading}
            className="text-xs h-10 rounded-xl border-slate-200 dark:border-slate-800"
          >
            Batal
          </Button>

          <Button
            type="button"
            onClick={handleSubmit}
            disabled={loading || !projectName.trim()}
            className="text-xs h-10 px-5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-600/20 flex items-center gap-2"
          >
            {loading ? (
              <>
                <span className="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Membuat Project & Pipeline...</span>
              </>
            ) : (
              <>
                <Sparkles className="h-4 w-4" />
                <span>Buat Creator Project Sekarang</span>
                <ArrowRight className="h-4 w-4" />
              </>
            )}
          </Button>
        </div>
      </div>
    </div>
  )
}
