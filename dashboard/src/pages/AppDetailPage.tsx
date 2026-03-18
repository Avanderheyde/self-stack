import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { getApp, getOperation, searchRegistry, startApp, stopApp, removeApp, updateApp, updatePort, streamLogs } from '../api';
import type { App } from '../api';
import StatusBadge from '../components/StatusBadge';
import { useAppUrl } from '../portless';
import { isNewerVersion } from '../version';

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
                  href={appUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-blue-600 hover:text-blue-800 transition-colors"
                >
                  {appUrl.replace('http://', '')}
                </a>
              )}
              {app.Status === 'stopped' && app.HostPort > 0 && !editingPort && (
                <button
                  onClick={() => { setPortValue(String(app.HostPort)); setEditingPort(true); }}
                  className="text-sm text-gray-500 hover:text-gray-700 transition-colors"
                >
                  Port {app.HostPort}
                </button>
              )}
              {app.Status === 'stopped' && editingPort && (
                <form
                  className="flex items-center gap-1"
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
                    className="w-24 text-sm px-2 py-1 border border-gray-300 rounded-md"
                  />
                  <button
                    type="submit"
                    disabled={busy}
                    className="text-sm px-2 py-1 rounded-md bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50"
                  >
                    Update
                  </button>
                  <button
                    type="button"
                    onClick={() => setEditingPort(false)}
                    className="text-sm px-2 py-1 text-gray-500 hover:text-gray-700"
                  >
                    Cancel
                  </button>
                </form>
              )}
              {app.Version && (
                <span className="text-xs text-gray-400">v{app.Version}</span>
              )}
            </div>
          </div>
        </div>

        {latestVersion && isNewerVersion(latestVersion, app.Version) && (
          <div className="mt-6 px-4 py-3 bg-amber-50 border border-amber-200 rounded-lg">
            <div className="flex items-center justify-between">
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
                  className="text-sm px-4 py-1.5 rounded-lg bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
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
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-xs font-medium text-amber-700">
                      {updateStep || 'Preparing...'}
                    </span>
                    <span className="text-xs text-amber-600">
                      {currentIdx >= 0 ? `${currentIdx + 1}/${steps.length}` : ''}
                    </span>
                  </div>
                  <div className="w-full h-2 bg-amber-200 rounded-full overflow-hidden">
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

        {!updating && !removing && <div className="flex gap-2 mt-6">
          {app.Status === 'running' && (
            <>
              <a
                href={appUrl}
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
            className="text-sm px-4 py-2 rounded-lg bg-red-50 text-red-700 hover:bg-red-100 disabled:opacity-50 transition-colors"
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
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs font-medium text-red-700">
                  {removeStep || 'Preparing...'}
                </span>
                <span className="text-xs text-red-500">
                  {currentIdx >= 0 ? `${currentIdx + 1}/${steps.length}` : ''}
                </span>
              </div>
              <div className="w-full h-2 bg-red-100 rounded-full overflow-hidden">
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
        <div className="mt-6 border border-gray-200 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-2">About</h2>
          <p className="text-sm text-gray-600">{app.Description}</p>
        </div>
      )}

      <div className="mt-6 border border-gray-200 rounded-xl p-6">
        <h2 className="text-sm font-semibold text-gray-900 mb-2">Configuration</h2>
        <p className="text-sm text-gray-400">Configuration options will appear here in a future update.</p>
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
    <div className="mt-6 border border-gray-200 rounded-xl p-6">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-gray-900">Logs</h2>
        {appStatus === 'running' && (
          <button
            onClick={streaming ? stop : start}
            className="text-xs px-3 py-1 rounded-md bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
          >
            {streaming ? 'Stop' : 'Stream Logs'}
          </button>
        )}
      </div>
      {lines.length === 0 && !streaming && (
        <p className="text-sm text-gray-400">
          {appStatus === 'running' ? 'Click "Stream Logs" to view container output.' : 'Start the app to view logs.'}
        </p>
      )}
      {(lines.length > 0 || streaming) && (
        <div className="bg-gray-950 text-gray-300 rounded-lg p-4 font-mono text-xs max-h-80 overflow-y-auto">
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
