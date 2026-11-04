import { FileText } from 'lucide-react'
import { InternalVisibility } from '../../components/ui/InternalVisibility'
import { useLanguage } from '../../context/LanguageContext'

export function ReplyDraftCard({ draft, internal, onUse }: { draft: string; internal: boolean; onUse: (draft: string) => void }) {
  const { t } = useLanguage()
  return (
    <div data-card="reply-draft" className="mt-2 w-full max-w-xl min-w-0 bg-card/40 text-foreground">
      <div className="flex items-center gap-2 px-3 pt-3 text-xs text-muted-foreground">
        {internal && <InternalVisibility className="-ml-3 -my-2" />}
        <FileText className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{t('Customer reply draft · Not sent yet')}</span>
      </div>
      <p className="px-3 py-3 text-sm leading-relaxed whitespace-pre-wrap break-words">{draft}</p>
      <div data-draft-footer className="border-t border-border/40 px-1">
        <button type="button" onClick={() => onUse(draft)} className="inline-flex min-h-11 items-center gap-2 px-2 text-xs font-medium text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring cursor-pointer">
          <FileText className="h-4 w-4" aria-hidden="true" />
          {t('Review & Use')}
        </button>
      </div>
    </div>
  )
}
