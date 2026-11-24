import { useEffect, useState, useMemo } from 'react'
import {
  GitCommit,
  GitBranch,
  Search,
  RefreshCw,
  Copy,
  Check,
  FileCode,
  ExternalLink,
  Clock,
  User,
  AlertCircle,
  FileText
} from 'lucide-react'
import { Link } from 'react-router-dom'
import { useLanguage } from '../../context/LanguageContext'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import {
  projectsApi,
  ProjectCommit,
  ProjectCommitDetail,
  ProjectGitLogResponse
} from '../../api/projects'
import type { Project } from '../../api/types'
import { relativeTime } from '../tasks/task-ui'

interface ProjectActivityViewProps {
  project: Project
  onRetry?: () => void
}

interface DateGroup {
  label: string
  commits: ProjectCommit[]
}

function formatDateHeader(dateStr: string, t: (key: string) => string): string {
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return t('Lainnya')

  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const targetDate = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  const diffDays = Math.round((today.getTime() - targetDate.getTime()) / (1000 * 60 * 60 * 24))

  if (diffDays === 0) return t('Hari Ini')
  if (diffDays === 1) return t('Kemarin')
  if (diffDays > 1 && diffDays < 7) {
    return d.toLocaleDateString(undefined, { weekday: 'long' })
  }
  return d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
}

export function ProjectActivityView({ project }: ProjectActivityViewProps) {
  const { t } = useLanguage()
  const [data, setData] = useState<ProjectGitLogResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [selectedSha, setSelectedSha] = useState<string | null>(null)
  const [commitDetail, setCommitDetail] = useState<ProjectCommitDetail | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [copiedSha, setCopiedSha] = useState(false)

  const loadLog = async (silent = false) => {
    if (!silent) setLoading(true)
    setError('')
    try {
      const res = await projectsApi.getGitLog(project.id, 50)
      setData(res)
    } catch (e: any) {
      setError(e?.message || t('Gagal memuat riwayat Git proyek.'))
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    void loadLog()
  }, [project.id])

  // Fetch commit details when SlideDrawer opens
  useEffect(() => {
    if (!selectedSha) {
      setCommitDetail(null)
      return
    }
    let cancelled = false
    setLoadingDetail(true)
    projectsApi.getCommitDetail(project.id, selectedSha)
      .then((detail) => {
        if (!cancelled) setCommitDetail(detail)
      })
      .catch((err) => {
        if (!cancelled) console.error('Failed to load commit detail:', err)
      })
      .finally(() => {
        if (!cancelled) setLoadingDetail(false)
      })
    return () => {
      cancelled = true
    }
  }, [selectedSha, project.id])

  const filteredCommits = useMemo(() => {
    if (!data?.commits) return []
    const q = query.toLowerCase().trim()
    if (!q) return data.commits
    return data.commits.filter(
      (c) =>
        c.message.toLowerCase().includes(q) ||
        c.short_sha.toLowerCase().includes(q) ||
        c.author_name.toLowerCase().includes(q) ||
        (c.task_ref && c.task_ref.toLowerCase().includes(q))
    )
  }, [data?.commits, query])

  const grouped = useMemo(() => {
    const groups: DateGroup[] = []
    let currentGroup: DateGroup | null = null

    for (const commit of filteredCommits) {
      const header = formatDateHeader(commit.date, t)
      if (!currentGroup || currentGroup.label !== header) {
        currentGroup = { label: header, commits: [] }
        groups.push(currentGroup)
      }
      currentGroup.commits.push(commit)
    }
    return groups
  }, [filteredCommits, t])

  const copyToClipboard = (text: string) => {
    if (!navigator?.clipboard) return
    void navigator.clipboard.writeText(text).then(() => {
      setCopiedSha(true)
      setTimeout(() => setCopiedSha(false), 2000)
    })
  }

  return (
    <div className="flex h-full flex-col overflow-hidden bg-background">
      {/* Top Bar: Search, Branch Badge, and Refresh */}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-border/40 px-4 py-3 sm:px-6">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 rounded-md bg-muted/60 px-2.5 py-1 text-xs font-medium text-foreground border border-border/40">
            <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="font-mono">{data?.branch || project.default_branch || 'main'}</span>
          </div>
          <span className="text-xs text-muted-foreground">
            {data?.total ? `${data.total} commit` : ''}
          </span>
        </div>

        <div className="flex items-center gap-2">
          <div className="relative w-48 sm:w-64">
            <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('Cari commit, pesan, atau task...')}
              className="h-8 w-full rounded-lg border border-border/50 bg-muted/30 pl-8 pr-3 text-xs text-foreground placeholder:text-muted-foreground/70 focus:border-primary focus:bg-background focus:outline-none"
            />
          </div>
          <button
            type="button"
            onClick={() => void loadLog(false)}
            disabled={loading}
            className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border/50 bg-muted/30 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors cursor-pointer"
            title={t('Muat ulang')}
            aria-label={t('Muat ulang')}
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex-1 overflow-y-auto px-4 py-4 sm:px-6">
        {loading && !data && (
          <div className="flex flex-col items-center justify-center py-16 text-center text-sm text-muted-foreground">
            <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground/60 mb-2" />
            <p>{t('Memuat riwayat commit...')}</p>
          </div>
        )}

        {error && (
          <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive flex items-start gap-3 my-4">
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <div>
              <p className="font-medium">{t('Gagal memuat log aktivitas')}</p>
              <p className="text-xs text-destructive/80 mt-0.5">{error}</p>
              <button
                type="button"
                onClick={() => void loadLog(false)}
                className="mt-2 text-xs font-semibold underline hover:no-underline cursor-pointer"
              >
                {t('Coba lagi')}
              </button>
            </div>
          </div>
        )}

        {!loading && !error && grouped.length === 0 && (
          <div className="flex flex-col items-center justify-center py-16 text-center text-sm text-muted-foreground">
            <GitCommit className="h-8 w-8 text-muted-foreground/40 mb-3" />
            <p className="font-medium text-foreground">{t('Belum ada aktivitas commit')}</p>
            <p className="max-w-xs text-xs text-muted-foreground/80 mt-1">
              {query
                ? t('Tidak ada commit yang cocok dengan pencarian.')
                : t('Perubahan kode yang dilakukan agen atau pengembang pada proyek ini akan tercatat di sini.')}
            </p>
          </div>
        )}

        {!loading && grouped.length > 0 && (
          <div className="space-y-6 max-w-4xl">
            {grouped.map((group) => (
              <div key={group.label} className="space-y-1.5">
                {/* Date Section Header */}
                <h3 className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider px-1">
                  {group.label}
                </h3>

                {/* Commits List for Date */}
                <div className="divide-y divide-border/20 rounded-xl border border-border/40 bg-card/50 overflow-hidden shadow-2xs">
                  {group.commits.map((commit) => (
                    <div
                      key={commit.sha}
                      onClick={() => setSelectedSha(commit.sha)}
                      className="group flex min-h-[46px] items-center justify-between gap-3 px-3.5 py-2.5 transition-colors hover:bg-muted/40 active:bg-muted/60 cursor-pointer select-none"
                    >
                      <div className="flex min-w-0 flex-1 items-center gap-2.5">
                        <GitCommit className="h-4 w-4 shrink-0 text-muted-foreground/70 group-hover:text-foreground transition-colors" />

                        {commit.task_ref && (
                          <span
                            className="inline-flex shrink-0 items-center rounded-md bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-primary border border-primary/20"
                            title={`Task ${commit.task_ref}`}
                          >
                            {commit.task_ref}
                          </span>
                        )}

                        <span className="truncate text-sm text-foreground group-hover:text-primary transition-colors">
                          {commit.message}
                        </span>
                      </div>

                      {/* Right Meta: Author, Relative Date, Short SHA */}
                      <div className="flex shrink-0 items-center gap-2.5 text-xs text-muted-foreground">
                        <span className="hidden sm:inline text-[11px]">
                          {commit.author_name}
                        </span>
                        <span className="text-[11px] text-muted-foreground/80 whitespace-nowrap">
                          {commit.relative_date || relativeTime(commit.date)}
                        </span>
                        <span
                          className="font-mono text-[11px] font-medium text-muted-foreground/90 bg-muted/70 px-1.5 py-0.5 rounded border border-border/30 hover:border-border transition-colors"
                          title={`Commit ${commit.sha}`}
                        >
                          {commit.short_sha}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Progressive Disclosure SlideDrawer for Commit Details */}
      <SlideDrawer
        isOpen={Boolean(selectedSha)}
        onClose={() => setSelectedSha(null)}
        title={commitDetail?.short_sha ? `Commit ${commitDetail.short_sha}` : t('Detail Commit')}
        description={commitDetail?.author_name ? `${commitDetail.author_name} · ${commitDetail.relative_date || ''}` : undefined}
        size="md"
      >
        <div className="space-y-5 p-5">
          {loadingDetail && !commitDetail && (
            <div className="flex flex-col items-center justify-center py-12 text-center text-sm text-muted-foreground">
              <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground/60 mb-2" />
              <p>{t('Memuat rincian commit...')}</p>
            </div>
          )}

          {commitDetail && (
            <>
              {/* Header Box: SHA & Copy, Author & Date */}
              <div className="rounded-xl border border-border/40 bg-muted/20 p-4 space-y-3">
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-muted-foreground">{t('Commit SHA')}</span>
                    <span className="font-mono text-xs font-semibold text-foreground select-all break-all">
                      {commitDetail.sha}
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => copyToClipboard(commitDetail.sha)}
                    className="inline-flex items-center gap-1 text-[11px] font-medium text-muted-foreground hover:text-foreground bg-muted/80 px-2 py-1 rounded border border-border/50 transition-colors cursor-pointer"
                    title={t('Salin SHA penuh')}
                  >
                    {copiedSha ? (
                      <>
                        <Check className="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
                        <span className="text-emerald-600 dark:text-emerald-400">{t('Tersalin')}</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-3 w-3" />
                        <span>{t('Salin')}</span>
                      </>
                    )}
                  </button>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 pt-1 border-t border-border/20 text-xs">
                  <div className="flex items-center gap-1.5 text-muted-foreground">
                    <User className="h-3.5 w-3.5 shrink-0" />
                    <span className="truncate">{commitDetail.author_name} &lt;{commitDetail.author_email}&gt;</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-muted-foreground">
                    <Clock className="h-3.5 w-3.5 shrink-0" />
                    <span>{new Date(commitDetail.date).toLocaleString()}</span>
                  </div>
                </div>
              </div>

              {/* Commit Message Box */}
              <div className="space-y-1.5">
                <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  {t('Pesan Commit')}
                </h4>
                <div className="rounded-xl border border-border/40 bg-card p-4 space-y-2">
                  <div className="flex items-start gap-2">
                    {commitDetail.task_ref && (
                      <span className="inline-flex shrink-0 items-center rounded-md bg-primary/10 px-2 py-0.5 font-mono text-xs font-semibold text-primary border border-primary/20">
                        {commitDetail.task_ref}
                      </span>
                    )}
                    <p className="text-sm font-medium text-foreground break-words leading-relaxed">
                      {commitDetail.message}
                    </p>
                  </div>
                  {commitDetail.body && (
                    <pre className="mt-2 whitespace-pre-wrap font-sans text-xs text-muted-foreground leading-relaxed pt-2 border-t border-border/20">
                      {commitDetail.body}
                    </pre>
                  )}
                  {commitDetail.task_id && (
                    <div className="pt-2">
                      <Link
                        to={`/tasks/${encodeURIComponent(commitDetail.task_id)}`}
                        className="inline-flex items-center gap-1.5 text-xs font-medium text-primary hover:underline"
                      >
                        <FileText className="h-3.5 w-3.5" />
                        <span>{t('Buka task terkait')} ({commitDetail.task_ref || commitDetail.task_id})</span>
                        <ExternalLink className="h-3 w-3 ml-0.5" />
                      </Link>
                    </div>
                  )}
                </div>
              </div>

              {/* Stats Summary & Modified Files */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    {t('Berkas yang Diubah')}
                  </h4>
                  {commitDetail.stats_summary && (
                    <span className="text-xs font-mono text-muted-foreground">
                      {commitDetail.stats_summary}
                    </span>
                  )}
                </div>

                {commitDetail.files && commitDetail.files.length > 0 ? (
                  <div className="rounded-xl border border-border/40 bg-card/60 divide-y divide-border/20 overflow-hidden">
                    {commitDetail.files.map((file) => (
                      <div
                        key={file.path}
                        className="flex items-center justify-between gap-3 px-3.5 py-2.5 text-xs"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <FileCode className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <span className="font-mono text-xs text-foreground truncate break-all">
                            {file.path}
                          </span>
                        </div>
                        <div className="flex items-center gap-2 shrink-0 font-mono text-[11px]">
                          {file.insertions > 0 && (
                            <span className="inline-flex items-center text-emerald-600 dark:text-emerald-400 font-medium">
                              +{file.insertions}
                            </span>
                          )}
                          {file.deletions > 0 && (
                            <span className="inline-flex items-center text-rose-600 dark:text-rose-400 font-medium">
                              -{file.deletions}
                            </span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-muted-foreground italic">
                    {t('Tidak ada berkas yang diubah.')}
                  </p>
                )}
              </div>

              {/* Patch Preview */}
              {commitDetail.patch_preview && (
                <div className="space-y-1.5 pt-2">
                  <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    {t('Preview Diff')}
                  </h4>
                  <pre className="max-h-80 overflow-auto rounded-xl border border-border/40 bg-muted/40 p-3 font-mono text-[11px] leading-relaxed text-muted-foreground select-text">
                    {commitDetail.patch_preview}
                  </pre>
                </div>
              )}
            </>
          )}
        </div>
      </SlideDrawer>
    </div>
  )
}
