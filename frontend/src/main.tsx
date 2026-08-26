import { QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { App } from './App'
import { ThemeProvider } from './context/ThemeProvider'
import { createQueryClient } from './lib/queryClient'
import './index.css'

const container = document.getElementById('root')
if (!container) throw new Error('Root element #root is missing')

createRoot(container).render(
  <StrictMode>
    <ThemeProvider>
      <QueryClientProvider client={createQueryClient()}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
)
