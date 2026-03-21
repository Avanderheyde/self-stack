import { useState } from 'react';
import { Link } from 'react-router';
import type { RegistryApp } from '../api';

function isUrl(s: string) {
  return s.startsWith('http://') || s.startsWith('https://');
}

function AppIcon({ app }: { app: RegistryApp }) {
  if (app.icon && isUrl(app.icon)) {
    return (
      <img
        src={app.icon}
        alt={app.display_name}
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
  const idx = app.name.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % gradients.length;
  return (
    <div className={`w-14 h-14 rounded-2xl bg-gradient-to-br ${gradients[idx]} flex items-center justify-center text-white font-bold text-xl shadow-sm`}>
      {(app.display_name || app.name).charAt(0).toUpperCase()}
    </div>
  );
}

export default function AppCard({
  app,
  installed,
  onInstall,
}: {
  app: RegistryApp;
  installed: boolean;
  onInstall: (name: string, onStep: (step: string) => void) => Promise<void>;
}) {
  const [installing, setInstalling] = useState(false);
  const [progressStep, setProgressStep] = useState('');

  const handleInstall = async () => {
    setInstalling(true);
    setProgressStep('');
    try {
      await onInstall(app.name, setProgressStep);
    } finally {
      setInstalling(false);
      setProgressStep('');
    }
  };

  return (
    <div className="group bg-white rounded-2xl p-5 border border-[var(--color-border)] hover:border-gray-300 hover:shadow-lg hover:shadow-gray-100 transition-all duration-200">
      <div className="flex items-start gap-4">
        <AppIcon app={app} />
        <div className="flex-1 min-w-0">
          <Link
            to={`/apps/${app.name}`}
            className="font-semibold text-[var(--color-text-primary)] hover:text-[var(--color-accent)] transition-colors text-[15px]"
          >
            {app.display_name || app.name}
          </Link>
          <p className="text-sm text-[var(--color-text-secondary)] mt-1 line-clamp-2 leading-relaxed">{app.description}</p>
        </div>
      </div>
      <div className="flex items-center justify-between mt-4 pt-4 border-t border-gray-100">
        <span className="text-xs px-2.5 py-1 rounded-full bg-gray-50 text-[var(--color-text-secondary)] font-medium">
          {app.category}
        </span>
        {installed ? (
          <span className="text-xs font-medium px-4 py-2 rounded-xl bg-emerald-50 text-emerald-600">
            Installed
          </span>
        ) : (
          <button
            onClick={handleInstall}
            disabled={installing}
            className="text-xs font-semibold px-5 py-2 rounded-xl bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 shadow-sm shadow-indigo-200"
          >
            {installing ? 'Installing...' : 'Install'}
          </button>
        )}
      </div>
      {installing && (() => {
        const steps = ['Fetching app info', 'Cloning repository', 'Building container', 'Starting app'];
        const currentIdx = progressStep ? steps.findIndex((s) => s === progressStep) : -1;
        const progress = currentIdx >= 0 ? ((currentIdx + 1) / steps.length) * 100 : 5;
        return (
          <div className="mt-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-[var(--color-text-secondary)]">
                {progressStep || 'Preparing...'}
              </span>
              <span className="text-xs text-[var(--color-text-muted)]">
                {currentIdx >= 0 ? `${currentIdx + 1}/${steps.length}` : ''}
              </span>
            </div>
            <div className="w-full h-1.5 bg-gray-100 rounded-full overflow-hidden">
              <div
                className="h-full bg-[var(--color-accent)] rounded-full transition-all duration-500 ease-out"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        );
      })()}
    </div>
  );
}
