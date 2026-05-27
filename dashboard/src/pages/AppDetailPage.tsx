import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { getApp, getOperation, searchRegistry, startApp, stopApp, removeApp, updateApp, updatePort, streamLogs } from '../api';
import type { App } from '../api';
import StatusBadge from '../components/StatusBadge';
import { useAppUrl } from '../portless';
import { useTailscaleAppUrl } from '../tailscale';
import { isNewerVersion } from '../version';

const sourceLabels: Record<string, string> = {
  registry: 'App Store',
  deploy: 'Deployed locally',
  local: 'Local source',
};

export default function AppDetailPage() {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const [app, setApp] = useState<App | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [iconUrl, setIconUrl] = useState<string | undefined>();
  const [editingPort, setEditingPort] = useState(false);
  const [portValue, setPortValue] = useState('');
  const [latestVersion, setLatestVersion] = useState<string | undefined>();
  const [updating, setUpdating] = useState(false);
  const [updateStep, setUpdateStep] = useState('');
  const [removeStep, setRemoveStep] = useState('');
  const appUrl = useAppUrl(app?.Name ?? '', app?.HostPort ?? 0);
  const tailscaleUrl = useTailscaleAppUrl(app?.HostPort ?? 0);

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
      if (entry?.version) {
        setLatestVersion(entry.version);
      }
    } catch {
      setApp(null);
    } finally {
      setLoading(false);
    }
  }, [name]);

  useEffect(() => { load(); }, [load]);

  // Poll for active background operations (e.g. update started before navigating away)
  useEffect(() => {
    if (!name || loading) return;
    let cancelled = false;
    const poll = async () => {
      try {
        const op = await getOperation(name);
        if (cancelled) return;
        if (op.active && op.type === 'update') {
          setUpdating(true);
          setUpdateStep(op.step);
          // Keep polling
          setTimeout(poll, 1000);
        } else if (op.done && op.type === 'update') {
          // Operation just finished
          setUpdating(false);
          setUpdateStep('');
          load();
        }
      } catch { /* ignore */ }
    };
    poll();
    return () => { cancelled = true; };
  }, [name, loading]); // eslint-disable-line react-hooks/exhaustive-deps

  const run = async (fn: (n: string) => Promise<void>) => {
    if (!name) return;
    setBusy(true);
    try { await fn(name); await load(); } finally { setBusy(false); }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-16">
        <div className="h-8 w-8 border-2 border-indigo-100 border-t-[var(--color-accent)] rounded-full animate-spin" />
      </div>
    );
  }

  if (!app) {
    return (
      <div className="text-center py-16">
        <p className="text-[var(--color-text-secondary)] font-medium">App not found</p>
        <Link to="/" className="text-sm text-[var(--color-accent)] hover:text-[var(--color-accent-hover)] mt-2 inline-block">
          Back to Store
        </Link>
      </div>
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

  const icon = iconUrl ? (
    <img src={iconUrl} alt={app.DisplayName || app.Name} className="w-16 h-16 rounded-2xl object-cover shadow-sm shrink-0" />
  ) : (
    <div className={`w-16 h-16 rounded-2xl bg-gradient-to-br ${gradients[idx]} flex items-center justify-center text-white font-bold text-2xl shadow-sm shrink-0`}>
      {(app.DisplayName || app.Name).charAt(0).toUpperCase()}
    </div>
  );

  return (
    <div>
      <Link to="/my-apps" className="text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors mb-6 inline-flex items-center gap-1.5">
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        My Apps
      </Link>

      <div className="mt-4 bg-white border border-[var(--color-border)] rounded-2xl p-6">
        <div className="flex items-start gap-5">
          {icon}
          <div className="flex-1 min-w-0">
            <h1 className="text-xl sm:text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">{app.DisplayName || app.Name}</h1>
            <div className="flex flex-wrap items-center gap-3 mt-2">
              <StatusBadge status={app.Status} />
              {app.Status === 'running' && app.HostPort > 0 && (
                <a
                  href={appUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-[var(--color-accent)] hover:text-[var(--color-accent-hover)] transition-colors"
                >
                  {appUrl.replace('http://', '')}
                </a>
              )}
              {app.Status === 'stopped' && app.HostPort > 0 && !editingPort && (
                <button
                  onClick={() => { setPortValue(String(app.HostPort)); setEditingPort(true); }}
                  className="text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] transition-colors"
                >
                  Port {app.HostPort}
                </button>
              )}
              {app.Status === 'stopped' && editingPort && (
                <form
                  className="flex items-center gap-1.5"
                  onSubmit={async (e) => {
                    e.preventDefault();
                    const p = parseInt(portValue, 10);
                    if (isNaN(p)) return;
                    setBusy(true);
                    try {
                      await updatePort(name!, p);
                      setEditingPort(false);
                      await load();
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <input
                    type="number"
                    min={1024}
                    max={65535}
                    value={portValue}
                    onChange={(e) => setPortValue(e.target.value)}
                    className="w-24 text-sm px-2.5 py-1.5 border border-[var(--color-border)] rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-100 focus:border-[var(--color-accent)]"
                  />
                  <button
                    type="submit"
                    disabled={busy}
                    className="text-sm px-3 py-1.5 rounded-lg bg-[var(--color-text-primary)] text-white hover:bg-gray-700 disabled:opacity-50 transition-colors"
                  >
                    Save
                  </button>
                  <button
                    type="button"
                    onClick={() => setEditingPort(false)}
                    className="text-sm px-3 py-1.5 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors"
                  >
                    Cancel
                  </button>
                </form>
              )}
              {app.Version && (
                <span className="text-xs text-[var(--color-text-muted)] font-mono bg-gray-50 px-2 py-0.5 rounded-md">v{app.Version}</span>
              )}
            </div>
          </div>
        </div>

        {latestVersion && isNewerVersion(latestVersion, app.Version) && (
          <div className="mt-6 px-4 py-3 bg-amber-50 border border-amber-100 rounded-xl">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm text-amber-800">
                Update available (v{app.Version || '0.0.0'} → v{latestVersion})
              </span>
              {!updating && (
                <button
                  onClick={async () => {
                    if (!name) return;
                    setUpdating(true);
                    setUpdateStep('');
                    try {
                      await updateApp(name, setUpdateStep);
                    } catch { /* SSE may disconnect on navigate, that's ok */ }
                    // Poll for completion
                    const pollDone = async () => {
                      try {
                        const op = await getOperation(name);
                        if (op.active) {
                          setUpdateStep(op.step);
                          setTimeout(pollDone, 1000);
                        } else {
                          setUpdating(false);
                          setUpdateStep('');
                          await load();
                        }
                      } catch {
                        setUpdating(false);
                        setUpdateStep('');
                      }
                    };
                    pollDone();
                  }}
                  disabled={busy}
                  className="text-sm px-4 py-1.5 rounded-xl bg-amber-500 text-white hover:bg-amber-600 disabled:opacity-50 transition-colors shrink-0"
                >
                  Update
                </button>
              )}
            </div>
            {updating && (() => {
              const steps = ['Stopping app', 'Pulling updates', 'Building container', 'Starting app'];
              const currentIdx = updateStep ? steps.findIndex((s) => s === updateStep) : -1;
              const progress = currentIdx >= 0 ? ((currentIdx + 1) / steps.length) * 100 : 5;
              return (
                <div className="mt-3">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs font-medium text-amber-700">
                      {updateStep || 'Preparing...'}
                    </span>
                    <span className="text-xs text-amber-600">
                      {currentIdx >= 0 ? `${currentIdx + 1}/${steps.length}` : ''}
                    </span>
                  </div>
                  <div className="w-full h-1.5 bg-amber-200 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-amber-500 rounded-full transition-all duration-500 ease-out"
                      style={{ width: `${progress}%` }}
                    />
                  </div>
                </div>
              );
            })()}
          </div>
        )}

        {!updating && !removing && <div className="flex flex-wrap gap-2 mt-6">
          {app.Status === 'running' && (
            <>
              <a
                href={appUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm font-medium px-5 py-2.5 rounded-xl bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:scale-95 transition-all duration-150 shadow-sm shadow-indigo-200"
              >
                Open App
              </a>
              <button
                onClick={() => run(stopApp)}
                disabled={busy}
                className="text-sm font-medium px-5 py-2.5 rounded-xl bg-gray-100 text-[var(--color-text-secondary)] hover:bg-gray-200 disabled:opacity-50 active:scale-95 transition-all duration-150"
              >
                Stop
              </button>
            </>
          )}
          {app.Status === 'stopped' && (
            <button
              onClick={() => run(startApp)}
              disabled={busy}
              className="text-sm font-medium px-5 py-2.5 rounded-xl bg-[var(--color-text-primary)] text-white hover:bg-gray-700 disabled:opacity-50 active:scale-95 transition-all duration-150"
            >
              Start
            </button>
          )}
          <button
            onClick={async () => {
              if (!name || !confirm(`Remove ${app.DisplayName || app.Name}?`)) return;
              setRemoving(true);
              setRemoveStep('');
              try {
                await removeApp(name, setRemoveStep);
                navigate('/my-apps');
              } catch {
                setRemoving(false);
                setRemoveStep('');
              }
            }}
            disabled={busy || removing}
            className="text-sm font-medium px-5 py-2.5 rounded-xl text-red-500 hover:bg-red-50 disabled:opacity-50 active:scale-95 transition-all duration-150"
          >
            {removing ? 'Removing...' : 'Remove'}
          </button>
        </div>}

        {removing && (() => {
          const steps = ['Stopping container', 'Cleaning up'];
          const currentIdx = removeStep ? steps.findIndex((s) => s === removeStep) : -1;
          const progress = currentIdx >= 0 ? ((currentIdx + 1) / steps.length) * 100 : 5;
          return (
            <div className="mt-4">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs font-medium text-red-700">
                  {removeStep || 'Preparing...'}
                </span>
                <span className="text-xs text-red-500">
                  {currentIdx >= 0 ? `${currentIdx + 1}/${steps.length}` : ''}
                </span>
              </div>
              <div className="w-full h-1.5 bg-red-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-red-500 rounded-full transition-all duration-500 ease-out"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
          );
        })()}
      </div>

      {app.Description && (
        <div className="mt-4 bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">About</h2>
          <p className="text-sm text-[var(--color-text-secondary)] leading-relaxed">{app.Description}</p>
        </div>
      )}

      <div className="mt-4 bg-white border border-[var(--color-border)] rounded-2xl p-6">
        <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-3">Access</h2>
        <dl className="text-sm space-y-2.5">
          <div className="flex items-baseline justify-between gap-4">
            <dt className="text-[var(--color-text-muted)]">Local</dt>
            <dd className="min-w-0">
              {app.HostPort > 0 ? (
                <a
                  href={appUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[var(--color-accent)] hover:text-[var(--color-accent-hover)] font-mono text-xs break-all"
                >
                  {appUrl}
                </a>
              ) : (
                <span className="text-[var(--color-text-muted)]">Not assigned</span>
              )}
            </dd>
          </div>
          <div className="flex items-baseline justify-between gap-4">
            <dt className="text-[var(--color-text-muted)]">Tailscale</dt>
            <dd className="min-w-0">
              {tailscaleUrl ? (
                <a
                  href={tailscaleUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[var(--color-accent)] hover:text-[var(--color-accent-hover)] font-mono text-xs break-all"
                >
                  {tailscaleUrl}
                </a>
              ) : (
                <span className="text-[var(--color-text-muted)] text-xs">
                  Run <code className="font-mono">tailscale serve --bg {app.HostPort || '<port>'}</code> to expose
                </span>
              )}
            </dd>
          </div>
          {app.SourceType && (
            <div className="flex items-baseline justify-between gap-4">
              <dt className="text-[var(--color-text-muted)]">Source</dt>
              <dd className="text-[var(--color-text-primary)] text-xs">
                {sourceLabels[app.SourceType] ?? app.SourceType}
                {app.RepoURL && (
                  <span className="ml-2 text-[var(--color-text-muted)] font-mono break-all">{app.RepoURL}</span>
                )}
              </dd>
            </div>
          )}
        </dl>
      </div>

      <LogViewer appName={name!} appStatus={app.Status} />
    </div>
  );
}

function LogViewer({ appName, appStatus }: { appName: string; appStatus: string }) {
  const [lines, setLines] = useState<string[]>([]);
  const [streaming, setStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const start = useCallback(() => {
    if (abortRef.current) abortRef.current.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setLines([]);
    setStreaming(true);
    streamLogs(appName, (line) => {
      setLines((prev) => {
        const next = [...prev, line];
        return next.length > 500 ? next.slice(-500) : next;
      });
    }, ac.signal);
    ac.signal.addEventListener('abort', () => setStreaming(false));
  }, [appName]);

  const stop = () => {
    abortRef.current?.abort();
    abortRef.current = null;
  };

  useEffect(() => () => stop(), []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  return (
    <div className="mt-4 bg-white border border-[var(--color-border)] rounded-2xl p-6">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">Logs</h2>
        {appStatus === 'running' && (
          <button
            onClick={streaming ? stop : start}
            className="text-xs font-medium px-3.5 py-1.5 rounded-lg bg-gray-100 text-[var(--color-text-secondary)] hover:bg-gray-200 active:scale-95 transition-all duration-150"
          >
            {streaming ? 'Stop' : 'Stream Logs'}
          </button>
        )}
      </div>
      {lines.length === 0 && !streaming && (
        <p className="text-sm text-[var(--color-text-muted)]">
          {appStatus === 'running' ? 'Click "Stream Logs" to view container output.' : 'Start the app to view logs.'}
        </p>
      )}
      {(lines.length > 0 || streaming) && (
        <div className="bg-gray-950 text-gray-300 rounded-xl p-4 font-mono text-xs max-h-80 overflow-y-auto">
          {lines.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all leading-5">{line}</div>
          ))}
          {streaming && lines.length === 0 && (
            <span className="text-gray-500">Waiting for logs...</span>
          )}
          <div ref={bottomRef} />
        </div>
      )}
    </div>
  );
}
