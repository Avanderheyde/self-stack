import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router';
import { checkForUpdate, applyUpdate, type UpdateCheck } from '../api';

const navLinks = [
  {
    to: '/',
    label: 'Store',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 21v-7.5a.75.75 0 0 1 .75-.75h3a.75.75 0 0 1 .75.75V21m-4.5 0H2.36m11.14 0H18m0 0h3.64m-1.39 0V9.349M3.75 21V9.349m0 0a1.5 1.5 0 0 1-.394-1.032l-.022-.378a1.5 1.5 0 0 1 .736-1.368l7.5-4.5a1.5 1.5 0 0 1 1.56 0l7.5 4.5a1.5 1.5 0 0 1 .736 1.368l-.022.378a1.5 1.5 0 0 1-.394 1.032" />
      </svg>
    ),
  },
  {
    to: '/my-apps',
    label: 'My Apps',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25a2.25 2.25 0 0 1-2.25-2.25v-2.25Z" />
      </svg>
    ),
  },
  {
    to: '/settings',
    label: 'Settings',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
      </svg>
    ),
  },
];

export default function Layout() {
  const [update, setUpdate] = useState<UpdateCheck | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applied, setApplied] = useState<string | null>(null);
  const location = useLocation();

  useEffect(() => {
    checkForUpdate().then(setUpdate).catch(() => {});
  }, []);

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
    <div className="min-h-screen bg-[var(--color-surface)]">
      {/* Desktop header */}
      <header className="hidden sm:block sticky top-0 z-40 bg-white/80 backdrop-blur-xl border-b border-[var(--color-border)]">
        <div className="mx-auto max-w-5xl px-6 flex items-center justify-between h-14">
          <div className="flex items-center gap-8">
            <span className="text-lg font-semibold tracking-tight text-[var(--color-text-primary)]">
              SelfStack
            </span>
            <nav className="flex gap-1">
              {navLinks.map(({ to, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === '/'}
                  className={({ isActive }) =>
                    `px-3 py-1.5 rounded-lg text-sm font-medium transition-all duration-150 ${
                      isActive
                        ? 'bg-[var(--color-accent)] text-white shadow-sm shadow-indigo-200'
                        : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-gray-100'
                    }`
                  }
                >
                  {label}
                </NavLink>
              ))}
            </nav>
          </div>
          {update && (
            <span className="text-xs text-[var(--color-text-muted)] font-mono">
              v{update.current}
            </span>
          )}
        </div>
      </header>

      {/* Mobile header */}
      <header className="sm:hidden sticky top-0 z-40 bg-white/80 backdrop-blur-xl border-b border-[var(--color-border)]">
        <div className="px-5 flex items-center justify-between h-12">
          <span className="text-lg font-semibold tracking-tight text-[var(--color-text-primary)]">
            SelfStack
          </span>
          {update && (
            <span className="text-xs text-[var(--color-text-muted)] font-mono">
              v{update.current}
            </span>
          )}
        </div>
      </header>

      {/* Update banner */}
      {showBanner && (
        <div className="bg-indigo-50 border-b border-indigo-100">
          <div className="mx-auto max-w-5xl px-5 sm:px-6 py-3 flex items-center justify-between gap-3">
            <span className="text-sm text-indigo-700">
              v{update.latest} available
            </span>
            <div className="flex gap-2 shrink-0">
              <button
                onClick={handleApply}
                disabled={applying}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] disabled:opacity-50 transition-colors"
              >
                {applying ? 'Updating...' : 'Update'}
              </button>
              <button
                onClick={() => setDismissed(true)}
                className="px-3 py-1.5 text-xs font-medium rounded-lg text-indigo-600 hover:bg-indigo-100 transition-colors"
              >
                Dismiss
              </button>
            </div>
          </div>
        </div>
      )}

      {applied && (
        <div className="bg-emerald-50 border-b border-emerald-100">
          <div className="mx-auto max-w-5xl px-5 sm:px-6 py-3">
            <span className="text-sm text-emerald-700">
              Updated to v{applied}. Restart the server to apply.
            </span>
          </div>
        </div>
      )}

      {/* Main content */}
      <main className="mx-auto max-w-5xl px-5 sm:px-6 py-6 sm:py-8 pb-24 sm:pb-8">
        <div key={location.pathname} className="animate-fade-in-up">
          <Outlet />
        </div>
      </main>

      {/* Mobile bottom nav */}
      <nav className="sm:hidden fixed bottom-0 inset-x-0 z-50 bg-white/90 backdrop-blur-xl border-t border-[var(--color-border)] pb-[env(safe-area-inset-bottom)]">
        <div className="flex justify-around items-center h-16">
          {navLinks.map(({ to, label, icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex flex-col items-center gap-1 px-4 py-2 rounded-xl transition-all duration-150 ${
                  isActive
                    ? 'text-[var(--color-accent)]'
                    : 'text-[var(--color-text-muted)]'
                }`
              }
            >
              {icon}
              <span className="text-[10px] font-medium">{label}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  );
}
