import { useEffect, useState } from 'react';
import { getStatus } from '../api';
import { useTailscale } from '../tailscale';
import { usePortless } from '../portless';

export default function SettingsPage() {
  const [version, setVersion] = useState('...');
  const [appCount, setAppCount] = useState(0);
  const tailscale = useTailscale();
  const portless = usePortless();

  useEffect(() => {
    getStatus().then((s) => {
      setVersion(s.version);
      setAppCount(s.apps);
    }).catch(() => {});
  }, []);

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">Settings</h1>
        <p className="text-sm text-[var(--color-text-secondary)] mt-1">System configuration and information</p>
      </div>

      <div className="space-y-4">
        <div className="bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">Tailscale</h2>
            <NetworkPill ok={!!tailscale.running} label={tailscale.running ? 'Connected' : tailscale.available ? 'Installed' : 'Not detected'} />
          </div>
          {tailscale.running ? (
            <dl className="text-sm space-y-2">
              {tailscale.dns_name && (
                <Row label="Node">
                  <span className="font-mono text-xs text-[var(--color-text-primary)]">{tailscale.dns_name}</span>
                </Row>
              )}
              {tailscale.ip && (
                <Row label="Tailnet IP">
                  <span className="font-mono text-xs text-[var(--color-text-primary)]">{tailscale.ip}</span>
                </Row>
              )}
              <Row label="Apps served">
                <span className="text-[var(--color-text-primary)] font-medium">{tailscale.served_ports?.length ?? 0}</span>
              </Row>
            </dl>
          ) : (
            <p className="text-sm text-[var(--color-text-muted)] leading-relaxed">
              {tailscale.available
                ? 'Tailscale is installed but not running. Start it with `tailscale up` to access apps from anywhere.'
                : 'Install Tailscale to access your apps from anywhere over an encrypted tailnet.'}
            </p>
          )}
        </div>

        <div className="bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">Portless</h2>
            <NetworkPill ok={portless.available} label={portless.available ? 'Active' : 'Off'} />
          </div>
          {portless.available ? (
            <p className="text-sm text-[var(--color-text-muted)] leading-relaxed">
              Apps are reachable at <code className="font-mono text-xs bg-gray-50 px-1.5 py-0.5 rounded">&lt;name&gt;.localhost:{portless.port}</code> — no port juggling.
            </p>
          ) : (
            <p className="text-sm text-[var(--color-text-muted)] leading-relaxed">
              Install <code className="font-mono text-xs bg-gray-50 px-1.5 py-0.5 rounded">portless</code> for named local URLs like <code className="font-mono text-xs bg-gray-50 px-1.5 py-0.5 rounded">notes.localhost</code>.
            </p>
          )}
        </div>

        <div className="bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-4">About</h2>
          <div className="space-y-3 text-sm">
            <Row label="Version">
              <span className="font-medium font-mono text-xs bg-gray-50 px-2.5 py-1 rounded-lg">{version}</span>
            </Row>
            <Row label="Installed Apps">
              <span className="font-medium">{appCount}</span>
            </Row>
            <Row label="Project">
              <span className="font-medium">SelfStack</span>
            </Row>
          </div>
        </div>
      </div>
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between items-center gap-3">
      <dt className="text-[var(--color-text-secondary)]">{label}</dt>
      <dd className="text-[var(--color-text-primary)] min-w-0 text-right">{children}</dd>
    </div>
  );
}

function NetworkPill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full ${
        ok ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-50 text-gray-600'
      }`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${ok ? 'bg-emerald-500' : 'bg-gray-400'}`} />
      {label}
    </span>
  );
}
