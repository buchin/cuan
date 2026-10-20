import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link, useLocation } from 'react-router-dom'
import { Search } from 'lucide-react'
import { ActionDialog } from '../dialogs/ActionDialog'
import { apiFetch } from '../../api/client'
import { useLanguage } from '../../context/LanguageContext'

type Item = { id: string; title: string; snippet: string; url: string }
type Group = { kind: string; items: Item[]; has_more: boolean; error?: string }
const labels: Record<string, string> = {
  contacts: 'Contacts',
  notes: 'Notes',
  conversations: 'Conversations',
  projects: 'Projects',
  tasks: 'Tasks',
}
function localResultURL(url: string) {
  return (
    /^\/(contacts\?|inbox\/|projects\/|tasks\/)/.test(url) &&
    !url.includes('\\')
  )
}
export function UniversalSearch() {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [results, setResults] = useState<{ query: string; groups: Group[] }>({
    query: '',
    groups: [],
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [revision, setRevision] = useState(0)
  const body = useRef<HTMLDivElement>(null)
  const location = useLocation()
  const term = query.trim()
  const groups = results.query === term ? results.groups : []
  useEffect(() => {
    setOpen(false)
  }, [location.pathname, location.search])
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        if (!open && document.querySelector('dialog[open]')) return
        event.preventDefault()
        event.stopPropagation()
        setOpen((value) => !value)
      }
    }
    window.addEventListener('keydown', handler, true)
    return () => window.removeEventListener('keydown', handler, true)
  }, [open])
  useEffect(() => {
    setOffset(0)
  }, [query, open])
  useEffect(() => {
    const controller = new AbortController()
    if (!open || Array.from(term).length < 2) {
      setLoading(false)
      setError('')
      return
    }
    setLoading(true)
    setError('')
    const timer = setTimeout(() => {
      apiFetch<{ groups: Group[] }>(
        '/api/v1/search?' +
          new URLSearchParams({ q: term, offset: String(offset) }),
        { signal: controller.signal },
      )
        .then((data) => {
          if (controller.signal.aborted) return
          setResults((old) => ({
            query: term,
            groups: data.groups.map((group) => {
              const previous =
                old.query === term
                  ? old.groups.find((value) => value.kind === group.kind)
                  : undefined
              return {
                ...group,
                items:
                  offset && previous
                    ? [
                        ...previous.items,
                        ...group.items.filter(
                          (item) =>
                            !previous.items.some(
                              (existing) => existing.id === item.id,
                            ),
                        ),
                      ]
                    : group.items,
              }
            }),
          }))
        })
        .catch(() => {
          if (!controller.signal.aborted)
            setError('Could not search. Try again.')
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false)
        })
    }, 250)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [open, term, offset, revision])
  const count = groups.reduce((total, group) => total + group.items.length, 0)
  return (
    <>
      <button
        type="button"
        className="icon-action"
        aria-label={t('Search Cuan')}
        aria-keyshortcuts="Control+k Meta+k"
        title={t('Search Cuan') + ' (⌘/Ctrl K)'}
        onClick={() => setOpen(true)}
      >
        <Search className="h-5 w-5" />
      </button>
      {createPortal(
        <ActionDialog
          open={open}
          title={t('Search Cuan')}
          onClose={() => setOpen(false)}
        >
          <div
            ref={body}
            className="space-y-4"
            onKeyDown={(event) => {
              const links = Array.from(
                body.current?.querySelectorAll<HTMLAnchorElement>(
                  '[data-search-result]',
                ) || [],
              )
              if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault()
                const index = links.indexOf(
                  document.activeElement as HTMLAnchorElement,
                )
                const next =
                  index < 0
                    ? event.key === 'ArrowDown'
                      ? 0
                      : links.length - 1
                    : (index +
                        (event.key === 'ArrowDown' ? 1 : -1) +
                        links.length) %
                      links.length
                links[next]?.focus()
              }
            }}
          >
            <label className="flex min-h-12 items-center gap-2 rounded-lg border px-3">
              <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
              <input
                autoComplete="off"
                type="search"
                aria-label={t('Search anything')}
                placeholder={t('Search anything…')}
                maxLength={200}
                className="min-w-0 w-full bg-transparent text-base outline-none"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    const first =
                      body.current?.querySelector<HTMLAnchorElement>(
                        '[data-search-result]',
                      )
                    if (first) {
                      event.preventDefault()
                      first.click()
                    }
                  }
                }}
              />
            </label>
            <p className="text-xs text-muted-foreground">
              {t('Contacts, notes, conversations, projects, and tasks.')}
            </p>
            <div
              role="status"
              aria-live="polite"
              className="text-xs text-muted-foreground"
            >
              {loading
                ? t('Searching…')
                : Array.from(term).length < 2
                  ? t('Type at least 2 characters.')
                  : !error &&
                      count === 0 &&
                      !groups.some((group) => group.error)
                    ? t('No results found.')
                    : ''}
            </div>
            {error && (
              <div role="alert" className="space-y-2 text-sm">
                <p>{t(error)}</p>
                <button
                  className="action-secondary"
                  onClick={() => setRevision((value) => value + 1)}
                >
                  {t('Try again')}
                </button>
              </div>
            )}
            {groups.map((group) => (
              <section
                key={group.kind}
                aria-label={t(labels[group.kind] || group.kind)}
                className="space-y-1"
              >
                {(group.items.length > 0 || group.error) && (
                  <h3 className="px-2 py-1 text-xs font-semibold text-muted-foreground">
                    {t(labels[group.kind] || group.kind)}
                  </h3>
                )}
                {group.error && (
                  <p className="px-2 text-xs text-destructive">
                    {t(group.error)}
                  </p>
                )}
                {group.items
                  .filter((item) => localResultURL(item.url))
                  .map((item) => (
                    <Link
                      key={item.id}
                      data-search-result
                      to={item.url}
                      onClick={() => setOpen(false)}
                      className="block min-h-14 rounded-md px-2 py-2.5 hover:bg-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
                    >
                      <span className="block truncate text-sm font-medium">
                        {item.title}
                      </span>
                      {item.snippet && (
                        <span className="mt-1 block line-clamp-2 break-words text-xs text-muted-foreground">
                          {item.snippet}
                        </span>
                      )}
                    </Link>
                  ))}
              </section>
            ))}
            {groups.some((group) => group.has_more) && (
              <button
                className="action-secondary w-full"
                disabled={loading}
                onClick={() => setOffset((value) => value + 5)}
              >
                {t('Show more results')}
              </button>
            )}
            {groups.some((group) => group.error) && !error && (
              <button
                className="action-secondary"
                disabled={loading}
                onClick={() => {
                  setOffset(0)
                  setRevision((value) => value + 1)
                }}
              >
                {t('Retry search')}
              </button>
            )}
          </div>
        </ActionDialog>,
        document.body,
      )}
    </>
  )
}
