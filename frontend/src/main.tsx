import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app/App'
import { installRefreshKeys } from '@/features/results/refreshKeys'
import '@/index.css'

const container = document.getElementById('root')
if (!container) throw new Error('#root missing from index.html')

installRefreshKeys()

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
