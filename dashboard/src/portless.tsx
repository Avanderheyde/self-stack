import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { getPortlessStatus, type PortlessStatus } from './api';

const PortlessContext = createContext<PortlessStatus>({ available: false, port: 0 });

export function PortlessProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<PortlessStatus>({ available: false, port: 0 });

  useEffect(() => {
    getPortlessStatus().then(setStatus).catch(() => {});
  }, []);

  return <PortlessContext value={status}>{children}</PortlessContext>;
}

export function usePortless() {
  return useContext(PortlessContext);
}

/** Returns the URL for an app, using portless if available. */
export function useAppUrl(appName: string, hostPort: number): string {
  const { available, port } = usePortless();
  if (available && port > 0) {
    return `http://${appName}.localhost:${port}`;
  }
  return `http://localhost:${hostPort}`;
}
