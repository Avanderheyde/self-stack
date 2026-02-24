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
        className="w-12 h-12 rounded-xl object-cover"
      />
    );
  }
  const colors = [
    'bg-blue-500', 'bg-green-500', 'bg-purple-500', 'bg-orange-500',
    'bg-pink-500', 'bg-teal-500', 'bg-indigo-500', 'bg-red-500',
  ];
  const idx = app.name.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % colors.length;
  return (
    <div className={`w-12 h-12 rounded-xl ${colors[idx]} flex items-center justify-center text-white font-bold text-lg`}>
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
    <div className="border border-gray-200 rounded-xl p-5 hover:shadow-md transition-shadow">
      <div className="flex items-start gap-4">
        <AppIcon app={app} />
        <div className="flex-1 min-w-0">
          <Link
            to={`/apps/${app.name}`}
            className="font-semibold text-gray-900 hover:text-gray-600 transition-colors"
          >
            {app.display_name || app.name}
          </Link>
          <p className="text-sm text-gray-500 mt-1 line-clamp-2">{app.description}</p>
        </div>
      </div>
      <div className="flex items-center justify-between mt-4">
        <span className="text-xs px-2.5 py-1 rounded-full bg-gray-100 text-gray-600">
          {app.category}
        </span>
        {installed ? (
          <span className="text-sm font-medium px-4 py-1.5 rounded-lg bg-gray-100 text-gray-500">
            Installed
          </span>
        ) : (
          <button
            onClick={handleInstall}
            disabled={installing}
            className="text-sm font-medium px-4 py-1.5 rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {installing ? (progressStep || 'Installing...') : 'Install'}
          </button>
        )}
      </div>
    </div>
  );
}
