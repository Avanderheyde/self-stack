import { useState } from 'react';
import { Link } from 'react-router';
import type { App } from '../api';
import StatusBadge from './StatusBadge';
import { useAppUrl } from '../portless';
import { useTailscaleAppUrl } from '../tailscale';
import { isNewerVersion } from '../version';

const sourceLabels: Record<string, string> = {
  deploy: 'Deployed',
  local: 'Local',
};

function AppIcon({ app, iconUrl }: { app: App; iconUrl?: string }) {
  if (iconUrl) {
    return (
      <img
        src={iconUrl}
        alt={app.DisplayName || app.Name}
        className="w-14 h-14 rounded-2xl object-cover shadow-sm"
      />
    );
  }
  const gradients = [
    'from-blue-500 to-blue-600',
    'from-emerald-500 to-emerald-600',
    'from-violet-500 to-violet-600',
    'from-orange-500 to-orange-600',
    'from-pink-500 to-pink-600',
    'from-teal-500 to-teal-600',
    'from-indigo-500 to-indigo-600',
    'from-rose-500 to-rose-600',
  ];
  const idx = app.Name.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % gradients.length;
  return (
    <div className={`w-14 h-14 rounded-2xl bg-gradient-to-br ${gradients[idx]} flex items-center justify-center text-white font-bold text-xl shadow-sm`}>
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
  const appUrl = useAppUrl(app.Name, app.HostPort);
  const tailscaleUrl = useTailscaleAppUrl(app.HostPort);
  const sourceLabel = sourceLabels[app.SourceType];

  const run = async (fn: (name: string) => Promise<void>) => {
    setBusy(true);
    try { await fn(app.Name); } finally { setBusy(false); }
  };

  return (
    <Link
      to={`/apps/${app.Name}`}
      className="block bg-white rounded-2xl p-5 border border-[var(--color-border)] hover:border-gray-300 hover:shadow-lg hover:shadow-gray-100 transition-all duration-200"
    >
      <div className="flex items-start gap-4">
        <div className="relative">
          <AppIcon app={app} iconUrl={iconUrl} />
          {latestVersion && isNewerVersion(latestVersion, app.Version) && (
            <span className="absolute -top-1 -right-1 w-3 h-3 rounded-full bg-amber-400 border-2 border-white" title="Update available" />
          )}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-[var(--color-text-primary)] text-[15px] truncate">
              {app.DisplayName || app.Name}
            </span>
            {sourceLabel && (
              <span className="shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded-md bg-gray-100 text-[var(--color-text-muted)] uppercase tracking-wide">
                {sourceLabel}
              </span>
            )}
          </div>
          <p className="text-sm text-[var(--color-text-secondary)] mt-1 line-clamp-2 leading-relaxed">{app.Description}</p>
        </div>
      </div>
      <div className="flex items-center justify-between mt-4 pt-4 border-t border-gray-100">
        <div className="flex items-center gap-2 min-w-0">
          <StatusBadge status={app.Status} />
          {app.Status === 'running' && app.HostPort > 0 && (
            <span className="text-xs text-[var(--color-accent)] font-medium truncate">
              {appUrl.replace(/^https?:\/\//, '')}
            </span>
          )}
          {app.Status === 'running' && tailscaleUrl && (
            <span
              title={`Reachable on tailnet: ${tailscaleUrl}`}
              className="shrink-0 text-[10px] font-semibold text-emerald-700 bg-emerald-50 border border-emerald-100 px-1.5 py-0.5 rounded-md"
            >
              TS
            </span>
          )}
        </div>
        <div className="flex gap-2" onClick={(e) => e.preventDefault()}>
          {app.Status === 'running' && (
            <>
              <a
                href={appUrl}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
                className="text-xs font-medium px-3.5 py-2 rounded-xl bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:scale-95 transition-all duration-150 shadow-sm shadow-indigo-200"
              >
                Open
              </a>
              <button
                onClick={(e) => { e.stopPropagation(); run(onStop); }}
                disabled={busy}
                className="text-xs font-medium px-3.5 py-2 rounded-xl bg-gray-100 text-[var(--color-text-secondary)] hover:bg-gray-200 disabled:opacity-50 active:scale-95 transition-all duration-150"
              >
                Stop
              </button>
            </>
          )}
          {app.Status === 'stopped' && (
            <button
              onClick={(e) => { e.stopPropagation(); run(onStart); }}
              disabled={busy}
              className="text-xs font-medium px-3.5 py-2 rounded-xl bg-[var(--color-text-primary)] text-white hover:bg-gray-700 disabled:opacity-50 active:scale-95 transition-all duration-150"
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
            className="text-xs font-medium px-3.5 py-2 rounded-xl text-red-500 hover:bg-red-50 disabled:opacity-50 active:scale-95 transition-all duration-150"
          >
            Remove
          </button>
        </div>
      </div>
    </Link>
  );
}
