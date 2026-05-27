import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { getTailscaleStatus, type TailscaleStatus } from './api';

const defaultStatus: TailscaleStatus = { available: false };
const TailscaleContext = createContext<TailscaleStatus>(defaultStatus);

export function TailscaleProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<TailscaleStatus>(defaultStatus);

  useEffect(() => {
    getTailscaleStatus().then(setStatus).catch(() => {});
  }, []);

  return <TailscaleContext value={status}>{children}</TailscaleContext>;
}

export function useTailscale() {
  return useContext(TailscaleContext);
}

/** Returns the Tailscale Serve URL for a host port, or null when unavailable. */
export function useTailscaleAppUrl(hostPort: number): string | null {
  const ts = useTailscale();
  if (!ts.running || !ts.dns_name || hostPort <= 0) return null;
  if (!ts.served_ports?.includes(hostPort)) return null;
  return `https://${ts.dns_name}:${hostPort}`;
}
