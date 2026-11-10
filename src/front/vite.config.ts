/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// localhost:8080 is the API in the dev container, or nginx when the compose stack runs on the host.
const API = 'http://localhost:8080'

// The origin the API is told the browser uses: its PUBLIC_URL. In the dev container the API and Web entries
// (.vscode/launch.json) both set PUBLIC_URL=http://localhost:5173, so OAuth's callback and the redirect after linking
// land here on Vite. On the host, `npm run dev` against the compose stack leaves it unset, and the stack's PUBLIC_URL
// is http://localhost:8080.
const PUBLIC_ORIGIN = process.env.PUBLIC_URL?.replace(/\/+$/, '') || API

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Bind IPv4 loopback: Node resolves localhost to ::1 only, which VS Code's dev container port forwarding can't reach.
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': {
        target: API,
        // The API refuses writes whose Origin isn't its PUBLIC_URL. The page is on :5173, which is PUBLIC_URL only in
        // the dev container, so present PUBLIC_ORIGIN instead. Production is one origin behind nginx and needs nothing.
        configure: (proxy) => {
          proxy.on('proxyReq', (req) => {
            if (req.getHeader('origin')) req.setHeader('origin', PUBLIC_ORIGIN)
          })
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    // Stylesheets stay unprocessed, except ?raw imports: the theme tests read them as text.
    css: { include: [/\.css\?raw$/] },
  },
})
