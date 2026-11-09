import { QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router/dom'
import { router } from './app/router'
import { createQueryClient } from './data/queryClient'
import { startTheme } from './theme/theme'
import '@fontsource-variable/big-shoulders-display'
import '@fontsource-variable/public-sans'
import '@fontsource-variable/red-hat-mono'
import './styles/index.css'

startTheme()
const queryClient = createQueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
