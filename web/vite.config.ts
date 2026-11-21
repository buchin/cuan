import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { mermaidAssets } from './mermaid-assets'

export default defineConfig({
  plugins: [react(), mermaidAssets()],
  build: {
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (
              id.includes('react-markdown') ||
              id.includes('remark-gfm') ||
              id.includes('micromark') ||
              id.includes('unist') ||
              id.includes('mdast') ||
              id.includes('hast') ||
              id.includes('vfile')
            ) {
              return 'markdown'
            }
            if (
              id.includes('node_modules/react/') ||
              id.includes('node_modules/react-dom/') ||
              id.includes('node_modules/react-router/') ||
              id.includes('node_modules/react-router-dom/')
            ) {
              return 'vendor-react'
            }
            if (id.includes('node_modules/lucide-react/')) {
              return 'vendor-icons'
            }
          }
        },
      },
    },
  },
})
