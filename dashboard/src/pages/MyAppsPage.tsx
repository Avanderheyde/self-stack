import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router';
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
        <div className="max-w-md mx-auto text-center py-12 sm:py-16">
          <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-indigo-50 to-violet-50 flex items-center justify-center mx-auto mb-4 border border-indigo-100">
            <svg className="w-8 h-8 text-[var(--color-accent)]" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
            </svg>
          </div>
          <p className="text-[var(--color-text-primary)] font-semibold">Nothing running yet</p>
          <p className="text-sm text-[var(--color-text-secondary)] mt-1.5 leading-relaxed">
            Install something from the Store, or deploy a local project with{' '}
            <code className="font-mono text-xs bg-gray-100 text-[var(--color-text-primary)] px-1.5 py-0.5 rounded">selfstack deploy</code>.
          </p>
          <Link
            to="/"
            className="inline-block mt-5 text-sm font-medium px-5 py-2.5 rounded-xl bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:scale-95 transition-all duration-150 shadow-sm shadow-indigo-200"
          >
            Browse the Store
          </Link>
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
