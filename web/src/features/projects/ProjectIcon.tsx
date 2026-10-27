import { Folder, GitBranch, Github } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export function ProjectIcon({ repoUrl }: { repoUrl?: string }) {
  const { t } = useLanguage()
  const github = /^https?:\/\/(?:www\.)?github\.com\//i.test(repoUrl || '') || /^git@github\.com:/i.test(repoUrl || '')
  const label = repoUrl ? (github ? 'GitHub repository' : 'Repository') : 'Project without a repository'
  const Badge = github ? Github : GitBranch
  return (
    <span role="img" aria-label={t(label)} title={t(label)} className="relative flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
      <Folder aria-hidden="true" className="h-4 w-4" />
      {repoUrl && <span className="absolute -bottom-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-full border border-border bg-card text-foreground">
        <Badge aria-hidden="true" className="h-2.5 w-2.5" />
      </span>}
    </span>
  )
}
