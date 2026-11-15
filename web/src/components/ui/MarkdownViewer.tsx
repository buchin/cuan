import { useState } from 'react'
import { Copy, Check, FileText } from 'lucide-react'
import { MermaidDiagram } from './MermaidDiagram'

interface MarkdownViewerProps {
  content: string
  className?: string
  showCopyButton?: boolean
}

interface Block {
  type:
    | 'h1'
    | 'h2'
    | 'h3'
    | 'h4'
    | 'p'
    | 'code'
    | 'quote'
    | 'ul'
    | 'ol'
    | 'hr'
    | 'table'
  text?: string
  lang?: string
  items?: string[]
  headers?: string[]
  rows?: string[][]
}

export function MarkdownViewer({
  content,
  className = '',
  showCopyButton = true,
}: MarkdownViewerProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback
      const textArea = document.createElement('textarea')
      textArea.value = content
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  // Parse lines into blocks
  const blocks: Block[] = []
  const lines = (content || '').split('\n')

  let inCodeBlock = false
  let codeBuffer: string[] = []
  let codeLang = ''

  let inTable = false
  let tableHeaders: string[] = []
  let tableRows: string[][] = []

  let listBuffer: string[] = []
  let listType: 'ul' | 'ol' | null = null

  const flushList = () => {
    if (listBuffer.length > 0 && listType) {
      blocks.push({ type: listType, items: [...listBuffer] })
      listBuffer = []
      listType = null
    }
  }

  const flushTable = () => {
    if (inTable) {
      blocks.push({
        type: 'table',
        headers: [...tableHeaders],
        rows: [...tableRows],
      })
      tableHeaders = []
      tableRows = []
      inTable = false
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const rawLine = lines[i]
    const trimmed = rawLine.trim()

    // Code blocks toggle
    if (trimmed.startsWith('```')) {
      if (inCodeBlock) {
        blocks.push({
          type: 'code',
          text: codeBuffer.join('\n'),
          lang: codeLang,
        })
        codeBuffer = []
        codeLang = ''
        inCodeBlock = false
      } else {
        flushList()
        flushTable()
        inCodeBlock = true
        codeLang = trimmed.slice(3).trim()
      }
      continue
    }

    if (inCodeBlock) {
      codeBuffer.push(rawLine)
      continue
    }

    // Markdown tables (| col1 | col2 |)
    if (trimmed.startsWith('|') && trimmed.endsWith('|')) {
      flushList()
      const cells = trimmed
        .slice(1, -1)
        .split('|')
        .map((c) => c.trim())

      // Divider row like |---|---|
      const isDivider = cells.every((c) => /^[-:\s]+$/.test(c))
      if (isDivider) {
        inTable = true
        continue
      }

      if (!inTable && tableHeaders.length === 0) {
        tableHeaders = cells
      } else {
        tableRows.push(cells)
      }
      continue
    } else if (inTable) {
      flushTable()
    }

    // Unordered lists (- or *)
    if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
      if (listType !== 'ul') flushList()
      listType = 'ul'
      listBuffer.push(trimmed.slice(2).trim())
      continue
    }

    // Ordered lists (1. or 2.)
    const olMatch = trimmed.match(/^(\d+)\.\s+(.*)$/)
    if (olMatch) {
      if (listType !== 'ol') flushList()
      listType = 'ol'
      listBuffer.push(olMatch[2].trim())
      continue
    }

    // If not list, flush ongoing list
    flushList()

    // Horizontal rule
    if (trimmed === '---' || trimmed === '***' || trimmed === '___') {
      blocks.push({ type: 'hr' })
      continue
    }

    // Headings
    if (trimmed.startsWith('# ')) {
      blocks.push({ type: 'h1', text: trimmed.slice(2) })
    } else if (trimmed.startsWith('## ')) {
      blocks.push({ type: 'h2', text: trimmed.slice(3) })
    } else if (trimmed.startsWith('### ')) {
      blocks.push({ type: 'h3', text: trimmed.slice(4) })
    } else if (trimmed.startsWith('#### ')) {
      blocks.push({ type: 'h4', text: trimmed.slice(5) })
    } else if (trimmed.startsWith('> ')) {
      blocks.push({ type: 'quote', text: trimmed.slice(2) })
    } else if (trimmed !== '') {
      blocks.push({ type: 'p', text: rawLine })
    }
  }

  // Flush remaining buffers
  flushList()
  flushTable()
  if (inCodeBlock && codeBuffer.length > 0) {
    blocks.push({ type: 'code', text: codeBuffer.join('\n'), lang: codeLang })
  }

  // Render inline formatting: **bold**, *italic*, `code`, [link](url), [ ] checkbox
  const renderInline = (str: string) => {
    // Checkbox checklist format: [ ] or [x]
    if (str.startsWith('[ ] ')) {
      return (
        <span className="flex items-center gap-2 text-slate-700 dark:text-slate-300">
          <input
            type="checkbox"
            disabled
            className="rounded text-emerald-600 h-3.5 w-3.5 mt-0.5"
          />
          <span>{renderInline(str.slice(4))}</span>
        </span>
      )
    }
    if (str.startsWith('[x] ') || str.startsWith('[X] ')) {
      return (
        <span className="flex items-center gap-2 text-slate-400 line-through">
          <input
            type="checkbox"
            checked
            disabled
            className="rounded text-emerald-600 h-3.5 w-3.5 mt-0.5"
          />
          <span>{renderInline(str.slice(4))}</span>
        </span>
      )
    }

    // Split on inline code, bold, italic, and links
    // Regex splits on: `code`, **bold**, *italic*, [text](url)
    const regex = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|\[[^\]]+\]\([^)]+\))/g
    const parts = str.split(regex)

    return parts.map((part, pIdx) => {
      if (!part) return null

      // Inline code
      if (part.startsWith('`') && part.endsWith('`') && part.length > 1) {
        return (
          <code
            key={pIdx}
            className="px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-emerald-600 dark:text-emerald-400 font-mono text-[12px]"
          >
            {part.slice(1, -1)}
          </code>
        )
      }

      // Bold
      if (part.startsWith('**') && part.endsWith('**') && part.length > 3) {
        return (
          <strong
            key={pIdx}
            className="font-bold text-slate-900 dark:text-white"
          >
            {part.slice(2, -2)}
          </strong>
        )
      }

      // Italic
      if (part.startsWith('*') && part.endsWith('*') && part.length > 2) {
        return (
          <em key={pIdx} className="italic text-slate-800 dark:text-slate-200">
            {part.slice(1, -1)}
          </em>
        )
      }

      // Link
      const linkMatch = part.match(/^\[([^\]]+)\]\(([^)]+)\)$/)
      if (linkMatch) {
        return (
          <a
            key={pIdx}
            href={linkMatch[2]}
            target="_blank"
            rel="noopener noreferrer"
            className="text-emerald-600 dark:text-emerald-400 underline hover:text-emerald-500"
          >
            {linkMatch[1]}
          </a>
        )
      }

      return <span key={pIdx}>{part}</span>
    })
  }

  return (
    <div
      className={`space-y-4 text-xs leading-relaxed text-slate-800 dark:text-slate-200 ${className}`}
    >
      {showCopyButton && (
        <div className="flex justify-end mb-2">
          <button
            type="button"
            onClick={handleCopy}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 transition-colors shadow-xs"
            title="Copy the entire content draft"
          >
            {copied ? (
              <>
                <Check className="h-3.5 w-3.5 text-emerald-500" />
                <span className="text-emerald-600 dark:text-emerald-400">
                  Copied to clipboard!
                </span>
              </>
            ) : (
              <>
                <Copy className="h-3.5 w-3.5 text-slate-500" />
                <span>Copy content draft</span>
              </>
            )}
          </button>
        </div>
      )}

      {blocks.map((block, idx) => {
        if (block.type === 'h1') {
          return (
            <h1
              key={idx}
              className="text-lg sm:text-xl font-black text-slate-900 dark:text-white pt-3 pb-1 border-b border-slate-200 dark:border-slate-800"
            >
              {block.text}
            </h1>
          )
        }
        if (block.type === 'h2') {
          return (
            <h2
              key={idx}
              className="text-base sm:text-lg font-extrabold text-slate-900 dark:text-white pt-2.5 pb-0.5 border-b border-slate-100 dark:border-slate-800/60"
            >
              {block.text}
            </h2>
          )
        }
        if (block.type === 'h3') {
          return (
            <h3
              key={idx}
              className="text-sm sm:text-base font-bold text-slate-900 dark:text-white pt-2"
            >
              {block.text}
            </h3>
          )
        }
        if (block.type === 'h4') {
          return (
            <h4
              key={idx}
              className="text-xs sm:text-sm font-bold text-slate-800 dark:text-slate-200 pt-1"
            >
              {block.text}
            </h4>
          )
        }
        if (block.type === 'quote') {
          return (
            <blockquote
              key={idx}
              className="border-l-4 border-emerald-500 pl-3.5 py-1.5 my-2 italic bg-slate-50/80 dark:bg-slate-900/60 rounded-r-lg text-slate-700 dark:text-slate-300"
            >
              {renderInline(block.text || '')}
            </blockquote>
          )
        }
        if (block.type === 'ul') {
          return (
            <ul
              key={idx}
              className="space-y-1.5 pl-4 list-disc marker:text-emerald-500 my-2"
            >
              {block.items?.map((item, itemIdx) => (
                <li key={itemIdx} className="leading-relaxed">
                  {renderInline(item)}
                </li>
              ))}
            </ul>
          )
        }
        if (block.type === 'ol') {
          return (
            <ol
              key={idx}
              className="space-y-1.5 pl-4 list-decimal marker:text-emerald-500 marker:font-semibold my-2"
            >
              {block.items?.map((item, itemIdx) => (
                <li key={itemIdx} className="leading-relaxed">
                  {renderInline(item)}
                </li>
              ))}
            </ol>
          )
        }
        if (block.type === 'hr') {
          return (
            <hr
              key={idx}
              className="my-4 border-slate-200 dark:border-slate-800"
            />
          )
        }
        if (block.type === 'table') {
          return (
            <div
              key={idx}
              className="my-3 overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-800"
            >
              <table className="w-full text-left text-xs border-collapse">
                {block.headers && block.headers.length > 0 && (
                  <thead className="bg-slate-100 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      {block.headers.map((h, hIdx) => (
                        <th
                          key={hIdx}
                          className="py-2 px-3 font-bold text-slate-800 dark:text-slate-200"
                        >
                          {renderInline(h)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                )}
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800/60 bg-white dark:bg-slate-950">
                  {block.rows?.map((row, rIdx) => (
                    <tr
                      key={rIdx}
                      className="hover:bg-slate-50 dark:hover:bg-slate-900/50"
                    >
                      {row.map((cell, cIdx) => (
                        <td
                          key={cIdx}
                          className="py-2 px-3 text-slate-700 dark:text-slate-300"
                        >
                          {renderInline(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        }
        if (block.type === 'code') {
          const lang = (block.lang || '').toLowerCase()
          const isMermaid = lang === 'mermaid' || lang === 'flowchart'
          if (isMermaid) {
            return <MermaidDiagram key={idx} code={block.text || ''} />
          }
          return (
            <div
              key={idx}
              className="my-3 rounded-xl overflow-hidden border border-slate-700/80 bg-slate-950 shadow-md"
            >
              <div className="flex items-center justify-between px-3.5 py-1.5 bg-slate-900/90 border-b border-slate-800 text-[11px] font-mono text-slate-400">
                <span className="flex items-center gap-1.5">
                  <FileText className="h-3.5 w-3.5 text-emerald-400" />
                  {block.lang || 'text'}
                </span>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(block.text || '')
                  }}
                  className="hover:text-white transition-colors flex items-center gap-1"
                  title="Copy code"
                >
                  <Copy className="h-3 w-3" />
                  <span>Copy</span>
                </button>
              </div>
              <pre className="p-3.5 overflow-x-auto text-emerald-400 font-mono text-[11px] leading-relaxed">
                <code>{block.text}</code>
              </pre>
            </div>
          )
        }
        return (
          <p key={idx} className="leading-relaxed">
            {renderInline(block.text || '')}
          </p>
        )
      })}
    </div>
  )
}
