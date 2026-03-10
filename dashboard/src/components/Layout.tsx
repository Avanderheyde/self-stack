import { useEffect, useState } from 'react';
import { NavLink, Outlet } from 'react-router';
import { checkForUpdate, applyUpdate, type UpdateCheck } from '../api';

const navLinks = [
  { to: '/', label: 'App Store' },
  { to: '/my-apps', label: 'My Apps' },
  { to: '/settings', label: 'Settings' },
];

export default function Layout() {
  const [update, setUpdate] = useState<UpdateCheck | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applied, setApplied] = useState<string | null>(null);

  useEffect(() => {
    checkForUpdate().then(setUpdate).catch(() => {});
  }, []);

  const currentVersion = update?.current ?? '…';

  const handleApply = async () => {
    setApplying(true);
    try {
      const res = await applyUpdate();
      if (res.updated) {
        setApplied(res.version);
      }
    } catch {
      // toast is already shown by apiFetch
    } finally {
      setApplying(false);
    }
  };

  const showBanner = update?.available && !dismissed && !applied;

  return (
    <div className="min-h-screen bg-white">
      <header className="border-b border-gray-200">
        <div className="mx-auto max-w-6xl px-6 flex items-center justify-between h-16">
          <span className="text-xl font-bold tracking-tight">SelfStack</span>
          <nav className="flex gap-1">
            {navLinks.map(({ to, label }) => (
              <NavLink
                key={to}
                to={to}
                end={to === '/'}
                className={({ isActive }) =>
                  `px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-gray-100 text-gray-900'
                      : 'text-gray-500 hover:text-gray-900 hover:bg-gray-50'
                  }`
                }
              >
                {label}
              </NavLink>
            ))}
          </nav>
          <span className="text-xs text-gray-400">v{currentVersion}</span>
        </div>
      </header>
      {showBanner && (
        <div className="bg-amber-50 border-b border-amber-200">
          <div className="mx-auto max-w-6xl px-6 py-3 flex items-center justify-between">
            <span className="text-sm text-amber-800">
              SelfStack v{update.latest} is available (current: v{update.current})
            </span>
            <div className="flex gap-2">
              <button
                onClick={handleApply}
                disabled={applying}
                className="px-3 py-1 text-sm font-medium rounded-md bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50"
              >
                {applying ? 'Updating…' : 'Update'}
              </button>
              <button
                onClick={() => setDismissed(true)}
                className="px-3 py-1 text-sm font-medium rounded-md text-amber-700 hover:bg-amber-100"
              >
                Dismiss
              </button>
            </div>
          </div>
        </div>
      )}
      {applied && (
        <div className="bg-green-50 border-b border-green-200">
          <div className="mx-auto max-w-6xl px-6 py-3">
            <span className="text-sm text-green-800">
              Updated! Restart the server to use v{applied}.
            </span>
          </div>
        </div>
      )}
      <main className="mx-auto max-w-6xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}
