export function isMarkdownFile(name: string): boolean {
  return /\.(md|markdown|mdown|mkdn)$/i.test(name)
}

export function isImageFile(name: string, type?: string): boolean {
  return (
    /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif)$/i.test(name) ||
    (type ? type.startsWith('image/') : false)
  )
}

export function isHtmlFile(name: string, type?: string): boolean {
  return /\.(html|htm)$/i.test(name) || (type ? type.includes('html') : false)
}

export function isSvgFile(name: string, type?: string): boolean {
  return /\.svg$/i.test(name) || (type ? type.includes('svg') : false)
}

export function isCsvFile(name: string, type?: string): boolean {
  return /\.(csv|tsv)$/i.test(name) || (type ? type.includes('csv') : false)
}

export function isDiffFile(name: string): boolean {
  return /\.(diff|patch)$/i.test(name)
}

export function isLogFile(name: string): boolean {
  return /\.(log)$/i.test(name)
}

export function isJsonFile(name: string, type?: string): boolean {
  return /\.json$/i.test(name) || (type ? type.includes('json') : false)
}

export function isCodeFile(name: string): boolean {
  return /\.(js|jsx|ts|tsx|mjs|cjs|go|py|rb|rs|c|cpp|h|hpp|java|kt|sh|bash|zsh|css|scss|sass|html|htm|sql|graphql|proto|yaml|yml|toml|xml)$/i.test(
    name,
  )
}

export function isTextFile(name: string, type?: string): boolean {
  return (
    /\.(txt|text|log|csv|tsv|env|ini|conf|config|diff|patch)$/i.test(name) ||
    (type ? type.startsWith('text/') : false)
  )
}

export function isPreviewableFile(name: string, type?: string): boolean {
  return (
    isMarkdownFile(name) ||
    isImageFile(name, type) ||
    isHtmlFile(name, type) ||
    isSvgFile(name, type) ||
    isCsvFile(name, type) ||
    isDiffFile(name) ||
    isLogFile(name) ||
    isJsonFile(name, type) ||
    isCodeFile(name) ||
    isTextFile(name, type)
  )
}

/**
 * Resolves an attachment URL to a reliable same-origin or canonical path.
 * Normalizes foreign/retired domains (such as legacy cuan.dojo.cc) into
 * same-origin relative endpoints so cross-origin, proxy, or CORS errors are avoided.
 */
export function resolveAttachmentUrl(a?: { id?: string; url?: string }): string {
  if (!a) return ''
  const rawUrl = a.url || ''
  if (a.id) {
    if (!rawUrl || rawUrl.includes('dojo.cc')) {
      return `/api/chat/attachments/${encodeURIComponent(a.id)}`
    }
    if (rawUrl.startsWith('http://') || rawUrl.startsWith('https://')) {
      try {
        const u = new URL(rawUrl)
        const isCurrentOrigin = typeof window !== 'undefined' && window.location && u.origin === window.location.origin
        if (!isCurrentOrigin) {
          return u.pathname + u.search
        }
      } catch {
        return `/api/chat/attachments/${encodeURIComponent(a.id)}`
      }
    }
  }
  return rawUrl || (a.id ? `/api/chat/attachments/${encodeURIComponent(a.id)}` : '')
}

