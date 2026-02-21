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

export async function installApp(name: string): Promise<App> {
  return apiFetch(`${API_BASE}/apps/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
}

export async function startApp(name: string): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/start`, { method: 'POST' });
}

export async function stopApp(name: string): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}/stop`, { method: 'POST' });
}

export async function removeApp(name: string): Promise<void> {
  await apiFetch(`${API_BASE}/apps/${encodeURIComponent(name)}`, { method: 'DELETE' });
}

export async function searchRegistry(query: string): Promise<RegistryApp[]> {
  return apiFetch(`${API_BASE}/registry/search?q=${encodeURIComponent(query)}`);
}

export async function getStatus(): Promise<{ version: string; apps: number; status: string }> {
  return apiFetch(`${API_BASE}/status`);
}
