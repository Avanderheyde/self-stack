import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import Dashboard from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Dashboard />
  </StrictMode>,
)
