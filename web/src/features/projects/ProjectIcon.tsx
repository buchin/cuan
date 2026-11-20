import { Folder, GitBranch, Github } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export function ProjectIcon({ repoUrl }: { repoUrl?: string }) {
  const { t } = useLanguage()
  const github = /^https?:\/\/(?:www\.)?github\.com\//i.test(repoUrl || '') || /^git@github\.com:/i.test(repoUrl || '')
  const label = repoUrl ? (github ? 'GitHub repository' : 'Repository') : 'Project without a repository'
  const Badge = github ? Github : GitBranch
  return (
    <span role="img" aria-label={t(label)} title={t(label)} className="relative flex h-8 w-8 shrink-0 items-center justify-center text-muted-foreground">
      <Folder aria-hidden="true" className="h-4 w-4" />
      {repoUrl && (
        <span className="absolute -bottom-0.5 -right-0.5 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-background border border-border/40 text-muted-foreground">
          <Badge aria-hidden="true" className="h-2 w-2" />
        </span>
      )}
    </span>
  )
}
