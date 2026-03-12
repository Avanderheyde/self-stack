import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import App from './App'
import { PortlessProvider } from './portless'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <PortlessProvider>
        <App />
      </PortlessProvider>
    </BrowserRouter>
  </StrictMode>,
)
