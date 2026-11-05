import { createReadStream, readFileSync, readdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { createRequire } from 'node:module'
import type { Plugin } from 'vite'

// Serve Mermaid's official prebuilt ESM distribution without parsing its whole
// diagram dependency tree during a Cuan build. Keep lazy relative chunk imports
// intact and host every file locally, pinned to the installed package version.
export function mermaidAssets(): Plugin {
  const require = createRequire(import.meta.url)
  const packageFile = require.resolve('mermaid/package.json')
  const directory = dirname(packageFile)
  const { version } = JSON.parse(readFileSync(packageFile, 'utf8')) as {
    version: string
  }
  if (!/^\d+\.\d+\.\d+$/.test(version))
    throw new Error('Unexpected Mermaid package version')
  const prefix = `assets/mermaid-${version}/`
  const files = new Map<string, string>([
    ['mermaid.esm.min.mjs', join(directory, 'dist/mermaid.esm.min.mjs')],
    ['LICENSE', join(directory, 'LICENSE')],
  ])
  const chunks = join(directory, 'dist/chunks/mermaid.esm.min')
  for (const file of readdirSync(chunks)) {
    if (/^[A-Za-z0-9_-]+\.mjs$/.test(file))
      files.set(`chunks/mermaid.esm.min/${file}`, join(chunks, file))
  }
  return {
    name: 'cuan-mermaid-assets',
    config: () => ({
      define: {
        __CUAN_MERMAID_URL__: JSON.stringify(`/${prefix}mermaid.esm.min.mjs`),
      },
      build: { rollupOptions: { external: [`/${prefix}mermaid.esm.min.mjs`] } },
    }),
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        const path = request.url?.split('?')[0] || ''
        if (!path.startsWith(`/${prefix}`)) return next()
        const file = files.get(path.slice(prefix.length + 1))
        if (!file) {
          response.statusCode = 404
          response.end()
          return
        }
        response.setHeader(
          'Content-Type',
          file.endsWith('.mjs') ? 'text/javascript' : 'text/plain',
        )
        response.setHeader('X-Content-Type-Options', 'nosniff')
        response.setHeader('Cache-Control', 'no-cache')
        createReadStream(file)
          .on('error', () => {
            response.statusCode = 500
            response.end()
          })
          .pipe(response)
      })
    },
    generateBundle() {
      for (const [relative, path] of files)
        this.emitFile({
          type: 'asset',
          fileName: prefix + relative,
          source: readFileSync(path),
        })
    },
  }
}
