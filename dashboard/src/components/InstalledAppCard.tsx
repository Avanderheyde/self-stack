import { useState } from 'react';
import { Link } from 'react-router';
import type { App } from '../api';
import StatusBadge from './StatusBadge';
import { isNewerVersion } from '../version';

function AppIcon({ app, iconUrl }: { app: App; iconUrl?: string }) {
  if (iconUrl) {
    return (
      <img
        src={iconUrl}
        alt={app.DisplayName || app.Name}
        className="w-12 h-12 rounded-xl object-cover"
      />
    );
  }
  const colors = [
    'bg-blue-500', 'bg-green-500', 'bg-purple-500', 'bg-orange-500',
    'bg-pink-500', 'bg-teal-500', 'bg-indigo-500', 'bg-red-500',
  ];
  const idx = app.Name.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % colors.length;
  return (
    <div className={`w-12 h-12 rounded-xl ${colors[idx]} flex items-center justify-center text-white font-bold text-lg`}>
      {(app.DisplayName || app.Name).charAt(0).toUpperCase()}
    </div>
  );
}

export default function InstalledAppCard({
  app,
  iconUrl,
  latestVersion,
  onStart,
  onStop,
  onRemove,
}: {
  app: App;
  iconUrl?: string;
  latestVersion?: string;
  onStart: (name: string) => Promise<void>;
  onStop: (name: string) => Promise<void>;
  onRemove: (name: string) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);

  const run = async (fn: (name: string) => Promise<void>) => {
    setBusy(true);
    try { await fn(app.Name); } finally { setBusy(false); }
  };

  return (
    <Link
      to={`/apps/${app.Name}`}
      className="block border border-gray-200 rounded-xl p-5 hover:shadow-md transition-shadow"
    >
      <div className="flex items-start gap-4">
        <AppIcon app={app} iconUrl={iconUrl} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-gray-900">
              {app.DisplayName || app.Name}
            </span>
            {latestVersion && isNewerVersion(latestVersion, app.Version) && (
              <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" title="Update available" />
            )}
          </div>
          <p className="text-sm text-gray-500 mt-1 line-clamp-2">{app.Description}</p>
        </div>
      </div>
      <div className="flex items-center justify-between mt-4">
        <div className="flex items-center gap-3">
          <StatusBadge status={app.Status} />
          {app.Status === 'running' && app.HostPort > 0 && (
            <span className="text-xs text-blue-600">
              :{app.HostPort}
            </span>
          )}
        </div>
        <div className="flex gap-2" onClick={(e) => e.preventDefault()}>
          {app.Status === 'running' && (
            <>
              <a
                href={`http://localhost:${app.HostPort}`}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
                className="text-xs px-3 py-1.5 rounded-lg bg-blue-50 text-blue-700 hover:bg-blue-100 transition-colors"
              >
                Open
              </a>
              <button
                onClick={(e) => { e.stopPropagation(); run(onStop); }}
                disabled={busy}
                className="text-xs px-3 py-1.5 rounded-lg bg-gray-100 text-gray-700 hover:bg-gray-200 disabled:opacity-50 transition-colors"
              >
                Stop
              </button>
            </>
          )}
          {app.Status === 'stopped' && (
            <button
              onClick={(e) => { e.stopPropagation(); run(onStart); }}
              disabled={busy}
              className="text-xs px-3 py-1.5 rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50 transition-colors"
            >
              Start
            </button>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              if (confirm(`Remove ${app.DisplayName || app.Name}?`)) run(onRemove);
            }}
            disabled={busy}
            className="text-xs px-3 py-1.5 rounded-lg bg-red-50 text-red-700 hover:bg-red-100 disabled:opacity-50 transition-colors"
          >
            Remove
          </button>
        </div>
      </div>
    </Link>
  );
}
