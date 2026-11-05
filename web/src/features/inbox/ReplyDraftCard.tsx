import { CardActions } from './CardActions'
import { FileText, RefreshCw } from 'lucide-react'
import { InternalVisibility } from '../../components/ui/InternalVisibility'
import { useLanguage } from '../../context/LanguageContext'
import { CollapsibleChatText } from './CollapsibleChatText'

export function ReplyDraftCard({ draft, internal, onUse, onRegenerate, regenerating = false, regenerateDisabled = false }: { draft: string; internal: boolean; onUse: (draft: string) => void; onRegenerate?: () => void; regenerating?: boolean; regenerateDisabled?: boolean }) {
  const { t } = useLanguage()
  return (
    <div data-card="reply-draft" className="inbox-attachment mt-2 bg-card/40 text-foreground">
      <div className="flex items-center gap-2 px-3 pt-3 text-xs text-muted-foreground">
        {internal && <InternalVisibility className="-ml-3 -my-2" />}
        <FileText className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{t('Customer reply draft · Not sent yet')}</span>
      </div>
      <div data-reply-draft-body className="px-3 py-3 text-sm leading-relaxed break-words">
        <CollapsibleChatText content={draft} hideTaskCard />
      </div>
      <CardActions data-draft-footer className="px-3">
        {onRegenerate && <button type="button" onClick={onRegenerate} disabled={regenerateDisabled || regenerating} aria-busy={regenerating} className="inline-flex min-h-11 items-center gap-2 text-xs font-medium text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring cursor-pointer disabled:opacity-50 disabled:cursor-default">
          <RefreshCw className={`h-4 w-4${regenerating ? ' animate-spin' : ''}`} aria-hidden="true" />
          {t(regenerating ? 'Regenerating…' : 'Regenerate')}
        </button>}
        <button type="button" data-primary-action onClick={() => onUse(draft)} className="inline-flex min-h-11 items-center gap-2 text-xs font-medium focus-visible:outline-2 focus-visible:outline-ring cursor-pointer">
          <FileText className="h-4 w-4" aria-hidden="true" />
          {t('Review & Use')}
        </button>
      </CardActions>
    </div>
  )
}
