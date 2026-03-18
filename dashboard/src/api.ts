import { toast } from './components/Toast';

const API_BASE = '/api';

export interface App {
  Name: string;
  DisplayName: string;
  Description: string;
  RepoURL: string;
  Version: string;
  HostPort: number;
  Status: string;
}

export interface RegistryApp {
  name: string;
  display_name: string;
  description: string;
  category: string;
  repo: string;
  icon: string;
  verified: boolean;
  version: string;
}

async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg = body?.error || `Request failed (${res.status})`;
    toast(msg);
    throw new Error(msg);
  }
  return res.json();
}

export async function getApps(): Promise<App[]> {
  return apiFetch(`${API_BASE}/apps`);
}

export async function getApp(name: string): Promise<App> {
  return apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}`);
}

async function readSSEStream(res: Response, onStep?: (step: string) => void): Promise<App> {
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  let result: App | null = null;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });

    let idx: number;
    while ((idx = buf.indexOf('\n\n')) !== -1) {
      const raw = buf.slice(0, idx).trim();
      buf = buf.slice(idx + 2);
      if (!raw.startsWith('data: ')) continue;
      const json = raw.slice(6);
      try {
        const evt = JSON.parse(json);
        if (evt.error) {
          toast(evt.error);
          throw new Error(evt.error);
        }
        if (evt.step && onStep) {
          onStep(evt.step);
        }
        if (evt.done && evt.app) {
          result = evt.app;
        }
      } catch (e) {
        if (e instanceof SyntaxError) continue;
        throw e;
      }
    }
  }

  if (!result) throw new Error('Stream ended without completion');
  return result;
}

export async function installApp(name: string, onStep?: (step: string) => void): Promise<App> {
  const res = await fetch(`${API_BASE}/apps/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg = body?.error || `Request failed (${res.status})`;
    toast(msg);
    throw new Error(msg);
  }
  return readSSEStream(res, onStep);
}

export function streamLogs(
  name: string,
  onLine: (line: string) => void,
  signal: AbortSignal,
): void {
  fetch(`${API_BASE}/apps/${encodeURIComponent(name)}/logs`, { signal })
    .then(async (res) => {
      if (!res.ok) return;
      const reader = res.body!.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        let idx: number;
        while ((idx = buf.indexOf('\n\n')) !== -1) {
          const raw = buf.slice(0, idx).trim();
          buf = buf.slice(idx + 2);
          if (!raw.startsWith('data: ')) continue;
          try {
            const evt = JSON.parse(raw.slice(6));
            if (evt.line != null) onLine(evt.line);
          } catch { /* ignore */ }
        }
      }
    })
    .catch(() => { /* aborted or network error */ });
}

export async function startApp(name: string): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/start`, { method: 'POST' });
}

export async function stopApp(name: string): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/stop`, { method: 'POST' });
}

export async function updatePort(name: string, port: number): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/port`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ port }),
  });
}

export async function updateApp(name: string, onStep?: (step: string) => void): Promise<App> {
  const res = await fetch(`${API_BASE}/apps/${encodeURIComponent(name)}/update`, { method: 'POST' });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg = body?.error || `Request failed (${res.status})`;
    toast(msg);
    throw new Error(msg);
  }
  return readSSEStream(res, onStep);
}

export async function removeApp(name: string, onStep?: (step: string) => void): Promise<void> {
  const res = await fetch(`${API_BASE}/apps/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const msg = body?.error || `Request failed (${res.status})`;
    toast(msg);
    throw new Error(msg);
  }
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf('\n\n')) !== -1) {
      const raw = buf.slice(0, idx).trim();
      buf = buf.slice(idx + 2);
      if (!raw.startsWith('data: ')) continue;
      try {
        const evt = JSON.parse(raw.slice(6));
        if (evt.error) { toast(evt.error); throw new Error(evt.error); }
        if (evt.step && onStep) onStep(evt.step);
      } catch (e) {
        if (e instanceof SyntaxError) continue;
        throw e;
      }
    }
  }
}

export async function searchRegistry(query: string): Promise<RegistryApp[]> {
  return apiFetch(`${API_BASE}/registry/search?q=${encodeURIComponent(query)}`);
}

export async function getStatus(): Promise<{ version: string; apps: number; status: string }> {
  return apiFetch(`${API_BASE}/status`);
}

export interface UpdateCheck {
  current: string;
  latest: string;
  available: boolean;
}

export async function checkForUpdate(): Promise<UpdateCheck> {
  const res = await fetch(`${API_BASE}/update/check`);
  if (!res.ok) throw new Error('update check failed');
  return res.json();
}

export async function applyUpdate(): Promise<{ updated: boolean; version: string }> {
  return apiFetch(`${API_BASE}/update/apply`, { method: 'POST' });
}

export interface AppOperation {
  active: boolean;
  type: string;
  step: string;
  done: boolean;
  error?: string;
}

export async function getOperation(name: string): Promise<AppOperation> {
  return apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/operation`);
}

export interface PortlessStatus {
  available: boolean;
  port: number;
}

export async function getPortlessStatus(): Promise<PortlessStatus> {
  return apiFetch(`${API_BASE}/portless`);
}
