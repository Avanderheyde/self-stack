import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router';
import { getApp, searchRegistry, startApp, stopApp, removeApp } from '../api';
import type { App } from '../api';
import StatusBadge from '../components/StatusBadge';

export default function AppDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [app, setApp] = useState<App | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [iconUrl, setIconUrl] = useState<string | undefined>();

  const load = useCallback(async () => {
    if (!name) return;
    setLoading(true);
    try {
      const [appData, registry] = await Promise.all([getApp(name), searchRegistry('')]);
      setApp(appData);
      const entry = registry.find((r) => r.name === name);
      if (entry?.icon && (entry.icon.startsWith('http://') || entry.icon.startsWith('https://'))) {
        setIconUrl(entry.icon);
      }
    } catch {
      setApp(null);
    } finally {
      setLoading(false);
    }
  }, [name]);

  useEffect(() => { load(); }, [load]);

  const run = async (fn: (n: string) => Promise<void>) => {
    if (!name) return;
    setBusy(true);
    try { await fn(name); await load(); } finally { setBusy(false); }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="h-6 w-6 border-2 border-gray-300 border-t-gray-600 rounded-full animate-spin" />
      </div>
    );
  }

  if (!app) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">App not found.</p>
        <Link to="/" className="text-sm text-blue-600 hover:text-blue-800 mt-2 inline-block">
          Back to App Store
        </Link>
      </div>
    );
  }

  const colors = [
    'bg-blue-500', 'bg-green-500', 'bg-purple-500', 'bg-orange-500',
    'bg-pink-500', 'bg-teal-500', 'bg-indigo-500', 'bg-red-500',
  ];
  const idx = app.Name.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % colors.length;

  const icon = iconUrl ? (
    <img src={iconUrl} alt={app.DisplayName || app.Name} className="w-16 h-16 rounded-xl object-cover shrink-0" />
  ) : (
    <div className={`w-16 h-16 rounded-xl ${colors[idx]} flex items-center justify-center text-white font-bold text-2xl shrink-0`}>
      {(app.DisplayName || app.Name).charAt(0).toUpperCase()}
    </div>
  );

  return (
    <div>
      <Link to="/my-apps" className="text-sm text-gray-500 hover:text-gray-900 transition-colors mb-6 inline-flex items-center gap-1">
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        Back to My Apps
      </Link>

      <div className="mt-6 border border-gray-200 rounded-xl p-6">
        <div className="flex items-start gap-5">
          {icon}
          <div className="flex-1">
            <h1 className="text-2xl font-semibold text-gray-900">{app.DisplayName || app.Name}</h1>
            <div className="flex items-center gap-3 mt-2">
              <StatusBadge status={app.Status} />
              {app.Status === 'running' && app.HostPort > 0 && (
                <a
                  href={`http://localhost:${app.HostPort}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-blue-600 hover:text-blue-800 transition-colors"
                >
                  localhost:{app.HostPort}
                </a>
              )}
              {app.Version && (
                <span className="text-xs text-gray-400">v{app.Version}</span>
              )}
            </div>
          </div>
        </div>

        <div className="flex gap-2 mt-6">
          {app.Status === 'running' && (
            <>
              <a
                href={`http://localhost:${app.HostPort}`}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm px-4 py-2 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors"
              >
                Open App
              </a>
              <button
                onClick={() => run(stopApp)}
                disabled={busy}
                className="text-sm px-4 py-2 rounded-lg bg-gray-100 text-gray-700 hover:bg-gray-200 disabled:opacity-50 transition-colors"
              >
                Stop
              </button>
            </>
          )}
          {app.Status === 'stopped' && (
            <button
              onClick={() => run(startApp)}
              disabled={busy}
              className="text-sm px-4 py-2 rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50 transition-colors"
            >
              Start
            </button>
          )}
          <button
            onClick={() => {
              if (confirm(`Remove ${app.DisplayName || app.Name}?`)) run(removeApp);
            }}
            disabled={busy}
            className="text-sm px-4 py-2 rounded-lg bg-red-50 text-red-700 hover:bg-red-100 disabled:opacity-50 transition-colors"
          >
            Remove
          </button>
        </div>
      </div>

      {app.Description && (
        <div className="mt-6 border border-gray-200 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-2">About</h2>
          <p className="text-sm text-gray-600">{app.Description}</p>
        </div>
      )}

      <div className="mt-6 border border-gray-200 rounded-xl p-6">
        <h2 className="text-sm font-semibold text-gray-900 mb-2">Configuration</h2>
        <p className="text-sm text-gray-400">Configuration options will appear here in a future update.</p>
      </div>

      <div className="mt-6 border border-gray-200 rounded-xl p-6">
        <h2 className="text-sm font-semibold text-gray-900 mb-2">Logs</h2>
        <p className="text-sm text-gray-400">Container logs will appear here in a future update.</p>
      </div>
    </div>
  );
}
