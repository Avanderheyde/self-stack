import { useEffect, useState } from 'react';
import { getStatus } from '../api';

export default function SettingsPage() {
  const [version, setVersion] = useState('...');
  const [appCount, setAppCount] = useState(0);

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
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">Network & Tunnels</h2>
          <p className="text-sm text-[var(--color-text-muted)]">
            Tunnel configuration and remote access settings will appear here in a future update.
          </p>
        </div>

        <div className="bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">Devices</h2>
          <p className="text-sm text-[var(--color-text-muted)]">
            Connected devices and pairing settings will appear here in a future update.
          </p>
        </div>

        <div className="bg-white border border-[var(--color-border)] rounded-2xl p-6">
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)] mb-4">About</h2>
          <div className="space-y-3 text-sm">
            <div className="flex justify-between items-center">
              <span className="text-[var(--color-text-secondary)]">Version</span>
              <span className="text-[var(--color-text-primary)] font-medium font-mono text-xs bg-gray-50 px-2.5 py-1 rounded-lg">{version}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-[var(--color-text-secondary)]">Installed Apps</span>
              <span className="text-[var(--color-text-primary)] font-medium">{appCount}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-[var(--color-text-secondary)]">Project</span>
              <span className="text-[var(--color-text-primary)] font-medium">SelfStack</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
