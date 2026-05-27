import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import App from './App'
import { PortlessProvider } from './portless'
import { TailscaleProvider } from './tailscale'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <PortlessProvider>
        <TailscaleProvider>
          <App />
        </TailscaleProvider>
      </PortlessProvider>
    </BrowserRouter>
  </StrictMode>,
)
