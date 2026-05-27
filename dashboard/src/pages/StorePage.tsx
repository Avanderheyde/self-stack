import { useCallback, useEffect, useState } from 'react';
import { searchRegistry, installApp, getApps } from '../api';
import type { RegistryApp } from '../api';
import SearchBar from '../components/SearchBar';
import AppCard from '../components/AppCard';

export default function StorePage() {
  const [apps, setApps] = useState<RegistryApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState<string | null>(null);
  const [installed, setInstalled] = useState<Set<string>>(new Set());

  const load = useCallback(async (q: string) => {
    setLoading(true);
    try {
      const [results, myApps] = await Promise.all([searchRegistry(q), getApps()]);
      setApps(results);
      setInstalled(new Set(myApps.map((a) => a.Name)));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(query); }, [query, load]);

  const categories = [...new Set(apps.map((a) => a.category).filter(Boolean))];
  const filtered = category ? apps.filter((a) => a.category === category) : apps;

  const handleInstall = async (name: string, onStep: (step: string) => void) => {
    await installApp(name, onStep);
    setInstalled((prev) => new Set(prev).add(name));
  };

  const handleSearch = useCallback((q: string) => setQuery(q), []);

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text-primary)] tracking-tight">App Store</h1>
        <p className="text-sm text-[var(--color-text-secondary)] mt-1">Discover and install self-hosted apps</p>
      </div>

      <div className="mb-6">
        <SearchBar onSearch={handleSearch} />
      </div>

      {categories.length > 0 && (
        <div className="flex gap-2 mb-6 flex-wrap">
          <button
            onClick={() => setCategory(null)}
            className={`text-xs px-3.5 py-1.5 rounded-full font-medium transition-all duration-150 ${
              !category
                ? 'bg-[var(--color-accent)] text-white shadow-sm shadow-indigo-200'
                : 'bg-white text-[var(--color-text-secondary)] border border-[var(--color-border)] hover:border-gray-300 hover:text-[var(--color-text-primary)]'
            }`}
          >
            All
          </button>
          {categories.map((cat) => (
            <button
              key={cat}
              onClick={() => setCategory(cat === category ? null : cat)}
              className={`text-xs px-3.5 py-1.5 rounded-full font-medium transition-all duration-150 ${
                cat === category
                  ? 'bg-[var(--color-accent)] text-white shadow-sm shadow-indigo-200'
                  : 'bg-white text-[var(--color-text-secondary)] border border-[var(--color-border)] hover:border-gray-300 hover:text-[var(--color-text-primary)]'
              }`}
            >
              {cat}
            </button>
          ))}
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-16">
          <div className="h-8 w-8 border-2 border-indigo-100 border-t-[var(--color-accent)] rounded-full animate-spin" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-16">
          <div className="text-4xl mb-3">?</div>
          <p className="text-[var(--color-text-secondary)] font-medium">No apps found</p>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">Try a different search term</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((app) => (
            <AppCard key={app.name} app={app} installed={installed.has(app.name)} onInstall={handleInstall} />
          ))}
        </div>
      )}
    </div>
  );
}
