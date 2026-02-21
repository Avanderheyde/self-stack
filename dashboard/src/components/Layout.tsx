import { NavLink, Outlet } from 'react-router';

const navLinks = [
  { to: '/', label: 'App Store' },
  { to: '/my-apps', label: 'My Apps' },
  { to: '/settings', label: 'Settings' },
];

export default function Layout() {
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
          <span className="text-xs text-gray-400">v0.1.0</span>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}
