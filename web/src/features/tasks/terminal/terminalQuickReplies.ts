export interface TerminalQuickReply {
  id: string
  title: string
  content: string
  shortcut?: string
}

export const TERMINAL_QUICK_REPLIES_STORAGE_KEY = 'cuan_terminal_quick_replies_v1'

export const DEFAULT_TERMINAL_QUICK_REPLIES: TerminalQuickReply[] = [
  {
    id: 'term-qr-continue',
    shortcut: 'lanjut',
    title: 'Lanjutkan',
    content: 'Lanjutkan langkah berikutnya dan verifikasi hasilnya.',
  },
  {
    id: 'term-qr-status',
    shortcut: 'status',
    title: 'Cek status',
    content: 'Jelaskan status progres saat ini dan kendala yang ada.',
  },
  {
    id: 'term-qr-test',
    shortcut: 'test',
    title: 'Jalankan test',
    content: 'Jalankan test terkait dan tampilkan hasilnya.',
  },
  {
    id: 'term-qr-fix',
    shortcut: 'fix',
    title: 'Perbaiki error',
    content: 'Analisis akar masalah error terakhir dan perbaiki.',
  },
]

function normalizeQuickReply(raw: unknown): TerminalQuickReply | null {
  if (!raw || typeof raw !== 'object') return null
  const item = raw as Record<string, unknown>
  const id = typeof item.id === 'string' ? item.id.trim() : ''
  const title = typeof item.title === 'string' ? item.title.trim() : ''
  const content = typeof item.content === 'string' ? item.content.trim() : ''
  const shortcut = typeof item.shortcut === 'string' ? item.shortcut.trim().replace(/^\/+/, '') : undefined
  if (!id || !title || !content) return null
  return shortcut ? { id, title, content, shortcut } : { id, title, content }
}

export function loadTerminalQuickReplies(): TerminalQuickReply[] {
  try {
    const raw = localStorage.getItem(TERMINAL_QUICK_REPLIES_STORAGE_KEY)
    if (raw === null) {
      return DEFAULT_TERMINAL_QUICK_REPLIES
    }
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return DEFAULT_TERMINAL_QUICK_REPLIES
    }
    return parsed
      .map(normalizeQuickReply)
      .filter((item): item is TerminalQuickReply => item !== null)
  } catch {
    return DEFAULT_TERMINAL_QUICK_REPLIES
  }
}

export function saveTerminalQuickReplies(items: TerminalQuickReply[]): void {
  try {
    localStorage.setItem(TERMINAL_QUICK_REPLIES_STORAGE_KEY, JSON.stringify(items))
  } catch {
    // ignore storage errors (e.g. private mode quota)
  }
}

export function filterTerminalQuickReplies(
  items: TerminalQuickReply[],
  query: string,
): TerminalQuickReply[] {
  const q = query.trim().toLowerCase()
  if (!q) return items
  return items.filter(
    (item) =>
      item.title.toLowerCase().includes(q) ||
      item.content.toLowerCase().includes(q) ||
      (item.shortcut && item.shortcut.toLowerCase().includes(q)),
  )
}
