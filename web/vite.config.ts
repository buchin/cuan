import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
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
        },
      },
    },
  },
})
