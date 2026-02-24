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
      <h1 className="text-2xl font-semibold text-gray-900 mb-6">My Apps</h1>

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="h-6 w-6 border-2 border-gray-300 border-t-gray-600 rounded-full animate-spin" />
        </div>
      ) : apps.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500">No apps installed.</p>
          <p className="text-sm text-gray-400 mt-1">Visit the App Store to get started.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
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
