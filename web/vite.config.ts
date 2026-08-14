import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../cmd/server/web/dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://127.0.0.1:13888' } },
})
