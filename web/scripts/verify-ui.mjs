import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { preview } from 'vite'
import { runMermaidChecks } from './mermaid-browser-checks.mjs'

const output = path.resolve(process.argv[2] || '')
if (!process.argv[2]) throw new Error('Provide a private output directory')
await fs.mkdir(output, { recursive: true, mode: 0o700 })
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
  // Browser workflows run after the diagram browser has closed. They render
  // actual application components with isolated API data, never Owner access.
  server.httpServer.closeAllConnections()
  await new Promise(resolve => server.httpServer.close(resolve))
  server = null
  const workflow = spawnSync(process.execPath, [
    fileURLToPath(new URL('./verify-inbox-work-panel.mjs', import.meta.url)),
    path.join(output, 'workflow'),
  ], { cwd: fileURLToPath(new URL('../..', import.meta.url)), stdio: 'inherit',
       env: { ...process.env, CUAN_WORKFLOW_NATIVE: '1', NODE_OPTIONS: '--max-old-space-size=384' } })
  if (workflow.error || workflow.status !== 0)
    throw new Error('Application workflow verification failed; diagrams alone do not establish UI coverage', {cause: workflow.error})
  captures.push('workflow/inbox-390.png', 'workflow/inbox-1440.png',
                'workflow/inbox-composer-390.png', 'workflow/inbox-composer-1440.png',
                'workflow/inbox-whisper-390.png', 'workflow/inbox-whisper-1440.png',
                'workflow/session-running-390.png', 'workflow/session-running-1440.png')
  for (const capture of captures) {
    const stat = await fs.stat(path.join(output, capture))
    if (!stat.isFile() || stat.size === 0) throw new Error(`Missing visual evidence: ${capture}`)
  }
  const components = [
    'web/src/features/inbox/InboxView.tsx',
    'web/src/features/inbox/ComposerTasks.tsx',
    'web/src/features/inbox/InboxWorkPanel.tsx',
    'web/src/components/ui/AccordionTimeline.tsx',
    'web/src/features/inbox/PromptChoiceChips.tsx',
    'web/src/features/tasks/TaskSessionPage.tsx',
    'web/src/features/tasks/TaskTerminal.tsx',
    'web/src/components/layout/AppShell.tsx',
    'web/src/components/ui/StatusDot.tsx',
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
