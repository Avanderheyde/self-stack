import { useCallback, useEffect, useState } from 'react';
import { getApps, searchRegistry, startApp, stopApp, removeApp } from '../api';
import type { App } from '../api';
import InstalledAppCard from '../components/InstalledAppCard';

export default function MyAppsPage() {
  const [apps, setApps] = useState<App[]>([]);
  const [loading, setLoading] = useState(true);
  const [icons, setIcons] = useState<Record<string, string>>({});
  const [latestVersions, setLatestVersions] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [myApps, registry] = await Promise.all([getApps(), searchRegistry('')]);
      setApps(myApps);
      const map: Record<string, string> = {};
      const versions: Record<string, string> = {};
      for (const r of registry) {
        if (r.icon && (r.icon.startsWith('http://') || r.icon.startsWith('https://'))) {
          map[r.name] = r.icon;
        }
        if (r.version) {
          versions[r.name] = r.version;
        }
      }
      setIcons(map);
      setLatestVersions(versions);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleStart = async (name: string) => { await startApp(name); await load(); };
  const handleStop = async (name: string) => { await stopApp(name); await load(); };
  const handleRemove = async (name: string) => { await removeApp(name); await load(); };

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">My Apps</h1>
        <p className="text-sm text-[var(--color-text-secondary)] mt-1">Manage your installed applications</p>
      </div>

      {loading ? (
        <div className="flex justify-center py-16">
          <div className="h-8 w-8 border-2 border-indigo-100 border-t-[var(--color-accent)] rounded-full animate-spin" />
        </div>
      ) : apps.length === 0 ? (
        <div className="text-center py-16">
          <div className="w-16 h-16 rounded-2xl bg-gray-100 flex items-center justify-center mx-auto mb-4">
            <svg className="w-8 h-8 text-[var(--color-text-muted)]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25 2.25M12 13.875l2.25-2.25M12 13.875l-2.25 2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
            </svg>
          </div>
          <p className="text-[var(--color-text-secondary)] font-medium">No apps installed yet</p>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">Head to the Store to get started</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {apps.map((app) => (
            <InstalledAppCard
              key={app.Name}
              app={app}
              iconUrl={icons[app.Name]}
              latestVersion={latestVersions[app.Name]}
              onStart={handleStart}
              onStop={handleStop}
              onRemove={handleRemove}
            />
          ))}
        </div>
      )}
    </div>
  );
}
