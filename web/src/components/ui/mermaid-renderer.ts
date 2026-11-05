// Mermaid is downloaded only when a diagram mounts. Serialize initialization
// and rendering because its theme/configuration is shared across diagrams.
let rendering: Promise<unknown> = Promise.resolve()
let sequence = 0

export function renderMermaid(
  source: string,
  dark: boolean,
): Promise<{ url: string; width: number; height: number }> {
  const result = rendering
    .catch(() => undefined)
    .then(async () => {
      if (source.length > 20_000)
        throw new Error('Diagram exceeds rendering budget')
      // SVG image nodes bypass Mermaid's HTML-label sanitizer and request their
      // URLs during layout. Exclude image metadata before handing it to Mermaid.
      // Decode YAML quoted-key escapes too; do not turn this into HTML filtering.
      const metadata = source
        .replace(
          /\\(?:u([a-fA-F\d]{4})|U([a-fA-F\d]{8})|x([a-fA-F\d]{2}))/g,
          (_, small, large, hex) => {
            const point = parseInt(small || large || hex, 16)
            return point <= 0x10ffff ? String.fromCodePoint(point) : ''
          },
        )
        .replace(/\\\r?\n[ \t]*/g, '')
      if (/@\s*\{/.test(metadata) && /\bimg["']?\s*:/i.test(metadata))
        throw new Error('Image nodes are not supported in private diagrams')
      if (
        /(?:^|\n)\s*(?:style|classDef|linkStyle)\b[^\n]*\burl\s*\(/i.test(
          metadata,
        )
      )
        throw new Error('External diagram styles are not supported')
      const { default: mermaid } = (await import(
        /* @vite-ignore */ __CUAN_MERMAID_URL__
      )) as { default: typeof import('mermaid').default }
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        htmlLabels: false,
        dompurifyConfig: {
          ALLOWED_TAGS: ['br', 'b', 'strong', 'i', 'em'],
          ALLOWED_ATTR: [],
        },
        maxTextSize: 20_000,
        maxEdges: 300,
        theme: 'base',
        fontFamily: 'Geist Variable, system-ui, sans-serif',
        flowchart: { htmlLabels: false, useMaxWidth: true },
        secure: [
          'secure',
          'securityLevel',
          'startOnLoad',
          'maxTextSize',
          'maxEdges',
          'theme',
          'themeCSS',
          'themeVariables',
          'flowchart',
          'fontFamily',
          'htmlLabels',
          'dompurifyConfig',
        ],
        themeVariables: {
          darkMode: dark,
          dropShadow: 'none',
          background: 'transparent',
          primaryColor: dark ? '#29292d' : '#f3f2ee',
          primaryTextColor: dark ? '#e4e4e7' : '#333338',
          primaryBorderColor: dark ? '#71717a' : '#a1a1aa',
          lineColor: dark ? '#a1a1aa' : '#71717a',
          secondaryColor: dark ? '#333338' : '#e9e8e4',
          tertiaryColor: dark ? '#242428' : '#f7f6f2',
          fontSize: '14px',
        },
      })
      const container = document.createElement('div')
      container.style.cssText =
        'position:fixed;left:-100000px;top:0;visibility:hidden'
      document.body.appendChild(container)
      try {
        const { svg } = await mermaid.render(
          `cuan-diagram-${++sequence}`,
          source,
          container,
        )
        const document = new DOMParser().parseFromString(svg, 'image/svg+xml')
        const element = document.documentElement
        const box = (element.getAttribute('viewBox') || '')
          .trim()
          .split(/[ ,]+/)
          .map(Number)
        if (
          box.length !== 4 ||
          !box.every(Number.isFinite) ||
          box[2] <= 0 ||
          box[3] <= 0
        )
          throw new Error('Diagram has no usable dimensions')
        const width = box[2],
          height = box[3]
        // Explicit dimensions retain natural text size in an image context;
        // width="100%" alone would stretch small diagrams to the bubble width.
        element.setAttribute('width', String(width))
        element.setAttribute('height', String(height))
        const rendered = new XMLSerializer().serializeToString(element)
        return {
          url: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(rendered)}`,
          width,
          height,
        }
      } finally {
        container.remove()
      }
    })
  rendering = result
  return result
}
