import { IconButton } from '../ui/IconButton'
import { Plus } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

// Desktop list headers keep creation beside the list title. Below this
// breakpoint the app header already owns the same action.
export function ListCreateAction({ label, onClick, kind }: {
  label: string
  onClick?: () => void
  kind: string
}) {
  const { t } = useLanguage()
  if (!onClick) return null
  const title = kind === 'projects' ? 'Projects' : kind === 'contacts' ? 'Contacts' : 'Inbox'
  return <div className="hidden min-h-11 items-center justify-between gap-3 pb-2 min-[1200px]:flex">
    <h2 className="text-sm font-semibold">{t(title)}</h2>
    <IconButton type="button" onClick={onClick} label={t(label)} title={t(label)}
      >
      <Plus className="h-4 w-4" aria-hidden="true" />
    </IconButton>
  </div>
}
