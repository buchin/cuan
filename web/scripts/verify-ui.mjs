import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { preview } from 'vite'
import { runMermaidChecks } from './mermaid-browser-checks.mjs'

const output = path.resolve(process.argv[2] || '')
if (!process.argv[2]) throw new Error('Provide a private output directory')
const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'cuan-ui-capture-'))
let server
try {
  // Build first, then start the browser. Keeping the compiler and Chromium
  // active together needlessly exceeds small runner memory allocations.
  const buildDirectory = path.join(temporary, 'build')
  const check = spawnSync(process.execPath, [
    fileURLToPath(new URL('./check-mermaid.mjs', import.meta.url)), buildDirectory,
  ], {
    stdio: 'inherit',
    env: { ...process.env, CUAN_DIAGRAM_BUILD_ONLY: '1', GOMAXPROCS: '2', NODE_OPTIONS: '--max-old-space-size=512' },
  })
  if (check.error || check.status !== 0) {
    throw new Error(`Real-component visual build failed (exit ${check.status}, signal ${check.signal || 'none'})`, { cause: check.error })
  }
  server = await preview({
    root: fileURLToPath(new URL('..', import.meta.url)),
    build: { outDir: buildDirectory },
    preview: { host: '127.0.0.1', port: 0, open: false },
    logLevel: 'error',
  })
  const fixturePath = (await fs.readFile(path.join(buildDirectory, 'fixture-path.txt'), 'utf8')).trim()
  const address = server.httpServer.address()
  await runMermaidChecks(`http://127.0.0.1:${address.port}${fixturePath}`, temporary)
  const captures = [
    'mermaid-390-light.png', 'mermaid-1440-light.png',
    'mermaid-viewer-390-light.png', 'mermaid-invalid-390.png',
  ]
  for (const name of captures) {
    await fs.copyFile(path.join(temporary, name), path.join(output, name), fs.constants.COPYFILE_EXCL)
  }
  const components = [
    'web/src/features/inbox/ChatMarkdown.tsx',
    'web/src/components/ui/MarkdownViewer.tsx',
    'web/src/components/ui/MermaidDiagram.tsx',
  ]
  await fs.writeFile(path.join(output, 'visual-evidence.json'),
    JSON.stringify({ components, captures }), { flag: 'wx', mode: 0o600 })
} finally {
  server?.httpServer.closeAllConnections()
  server?.httpServer.close()
  await fs.rm(temporary, { recursive: true, force: true })
}
