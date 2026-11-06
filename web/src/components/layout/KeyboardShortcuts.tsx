import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { trapDialogFocus } from '../../lib/dialog'

const destinations: Record<string, string> = { h: '/studio', i: '/inbox', t: '/tasks', p: '/projects', c: '/contacts', n: '/notifications', s: '/settings', a: '/account' }
const shortcuts = [['G then H', 'New work'], ['G then I', 'Inbox'], ['G then T', 'Tasks'], ['G then P', 'Projects'], ['G then C', 'Contacts'], ['G then N', 'Notifications'], ['G then S', 'Settings'], ['G then A', 'Account'], ['C', 'Focus message input'], ['N', 'Create in this page'], ['Ctrl / Cmd + K', 'Search everything'], ['?', 'Keyboard shortcuts']]

export function KeyboardShortcuts() {
  const navigate = useNavigate()
  const dialog = useRef<HTMLDialogElement>(null)
  const prefix = useRef(0)
  const prefixRoute = useRef('')
  useEffect(() => {
    const showHelp = () => { prefix.current = 0; if (!dialog.current?.open) dialog.current?.showModal() }
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing || event.repeat || event.ctrlKey || event.metaKey || event.altKey) return
      const target = event.target
      if (target instanceof Element && target.closest('input,textarea,select,[contenteditable]:not([contenteditable="false"]),[role="textbox"],.xterm,[data-hotkeys="off"]')) return
      if (document.querySelector('dialog[open],[role="dialog"][aria-modal="true"]')) { prefix.current = 0; return }
      const key = event.key.toLowerCase()
      if (event.key === '?') { event.preventDefault(); showHelp(); return }
      if (prefix.current > Date.now()) {
        const samePage=prefixRoute.current===window.location.pathname
        prefix.current = 0
        if (samePage && destinations[key]) { event.preventDefault(); navigate(destinations[key]); return }
      }
      if (key === 'g') { prefix.current = Date.now() + 1000; prefixRoute.current=window.location.pathname; return }
      if (key === 'c') {
        const input = document.querySelector<HTMLTextAreaElement>('[data-primary-composer]:not(:disabled)')
        if (input && input.getClientRects().length) { event.preventDefault(); input.focus() }
      }
      if (key === 'n') {
        const action = document.getElementById('header-new-action')
        if (action && action.getClientRects().length) { event.preventDefault(); action.click() }
      }
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('cuan:keyboard-shortcuts', showHelp)
    return () => { window.removeEventListener('keydown', onKey); window.removeEventListener('cuan:keyboard-shortcuts', showHelp) }
  }, [navigate])
  return <dialog ref={dialog} onKeyDown={trapDialogFocus} className="m-auto max-h-[85dvh] w-[calc(100%_-_2rem)] max-w-md overflow-y-auto rounded-xl border border-border bg-background p-5 text-foreground shadow-xl backdrop:bg-black/50" aria-labelledby="keyboard-shortcuts-title">
    <div className="mb-4 flex items-center justify-between gap-3"><h2 id="keyboard-shortcuts-title" className="text-base font-semibold">Keyboard shortcuts</h2><button type="button" autoFocus onClick={() => dialog.current?.close()} className="min-h-11 rounded-lg px-3 text-sm hover:bg-muted">Close</button></div>
    <p className="mb-3 text-sm text-muted-foreground">Navigation shortcuts work when you are not typing or using a terminal. Press Escape to close this guide.</p>
    <dl className="space-y-2">{shortcuts.map(([keys, label]) => <div key={keys} className="flex items-center justify-between gap-3 text-sm"><dt>{label}</dt><dd><kbd className="rounded border border-border bg-muted px-2 py-1 text-xs">{keys}</kbd></dd></div>)}</dl>
  </dialog>
}
