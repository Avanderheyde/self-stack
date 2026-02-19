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

export async function getApps(): Promise<App[]> {
  const res = await fetch(`${API_BASE}/apps`);
  return res.json();
}

export async function installApp(name: string): Promise<App> {
  const res = await fetch(`${API_BASE}/apps/install`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  return res.json();
}

export async function startApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}/start`, { method: 'POST' });
}

export async function stopApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}/stop`, { method: 'POST' });
}

export async function removeApp(name: string): Promise<void> {
  await fetch(`${API_BASE}/apps/${name}`, { method: 'DELETE' });
}

export async function searchRegistry(query: string): Promise<RegistryApp[]> {
  const res = await fetch(`${API_BASE}/registry/search?q=${encodeURIComponent(query)}`);
  return res.json();
}

export async function getStatus(): Promise<{ version: string; apps: number; status: string }> {
  const res = await fetch(`${API_BASE}/status`);
  return res.json();
}
