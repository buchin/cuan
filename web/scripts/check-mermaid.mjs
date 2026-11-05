// Real React/browser checks; temporary fixtures are removed before this exits.
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer, build } from 'vite'
import { flowSource, runMermaidChecks } from './mermaid-browser-checks.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const output = path.resolve(
  process.argv[2] || path.join(root, '../screenshots/mermaid'),
)
// Tailwind resolves its content globs relative to the build working directory.
process.chdir(root)
await fs.mkdir(output, { recursive: true })
const fixture = await fs.mkdtemp(path.join(root, '.mermaid-check-'))
const source = flowSource
const cases = {
  flow: source,
  sequence:
    'sequenceDiagram\n  User->>Cuan: Ada ide\n  Cuan-->>User: Hasil terverifikasi',
  invalid: 'flowchart TD\n  A --> [',
  attack:
    '%%{init: {"securityLevel":"loose"}}%%\nflowchart TD\n  A["<img src=https://diagram-external.invalid/image onerror=window.__diagramInjected=true>"] --> B[Safe]',
  remoteimage:
    'flowchart TD\n  A@{ img: "https://diagram-external.invalid/diagram.png", label: "Safe", h: 60 }',
  escapedimage:
    'flowchart TD\n  A@{ "i\\u006dg": "https://diagram-external.invalid/escaped.png", label: "Safe" }',
  remotestyle:
    'flowchart TD\n  A[Safe] --> B[Safe]\n  style A fill:url(https://diagram-external.invalid/style.svg)',
  oversized: 'flowchart TD\n' + ' '.repeat(20_001),
  none: '',
}
await fs.writeFile(
  path.join(fixture, 'index.html'),
  '<div id="root"></div><script type="module" src="./fixture.tsx"></script>',
)
await fs.writeFile(
  path.join(fixture, 'fixture.tsx'),
  `
import React from 'react'; import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import { ThemeProvider } from '/src/context/ThemeContext';
import { LanguageProvider } from '/src/context/LanguageContext';
import { ChatMarkdown } from '/src/features/inbox/ChatMarkdown';
import { MarkdownViewer } from '/src/components/ui/MarkdownViewer';
import '/src/index.css';
const params = new URLSearchParams(location.search);
const code = ${JSON.stringify(cases)}[params.get('kind') || 'flow'];
const content = code ? 'Berikut alurnya.\\n\\n\`\`\`mermaid\\n' + code + '\\n\`\`\`\\n\\nHasilnya dilaporkan di obrolan ini.' : 'Obrolan tanpa diagram.';
createRoot(document.getElementById('root')).render(<ThemeProvider><LanguageProvider><MemoryRouter><main className="mx-auto max-w-2xl p-5 text-foreground"><h1 className="text-xl font-semibold">Inbox</h1><section className="mt-5 min-w-0 rounded-xl bg-muted/20 p-4">{params.has('preview') ? <MarkdownViewer content={content} showCopyButton={false}/> : <ChatMarkdown content={content}/>}</section></main></MemoryRouter></LanguageProvider></ThemeProvider>);
`,
)
let server
try {
  if (process.env.CUAN_DIAGRAM_BUILD_ONLY === '1') {
    await build({
      root,
      build: {
        outDir: output,
        emptyOutDir: true,
        rollupOptions: { input: path.join(fixture, 'index.html') },
      },
    })
    await fs.writeFile(
      path.join(output, 'fixture-path.txt'),
      `/${path.basename(fixture)}/index.html`,
    )
  } else {
    server = await createServer({
      root,
      server: { host: '127.0.0.1', port: 0 },
      logLevel: 'error',
    })
    await server.listen()
    const address = server.httpServer.address()
    await runMermaidChecks(
      `http://127.0.0.1:${address.port}/${path.basename(fixture)}/index.html`,
      output,
    )
  }
} finally {
  await server?.close()
  await fs.rm(fixture, { recursive: true, force: true })
}
