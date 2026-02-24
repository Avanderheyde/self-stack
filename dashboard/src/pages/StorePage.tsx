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
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">App Store</h1>
      </div>

      <div className="mb-6 max-w-md">
        <SearchBar onSearch={handleSearch} />
      </div>

      {categories.length > 0 && (
        <div className="flex gap-2 mb-6 flex-wrap">
          <button
            onClick={() => setCategory(null)}
            className={`text-xs px-3 py-1.5 rounded-full transition-colors ${
              !category
                ? 'bg-gray-900 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            All
          </button>
          {categories.map((cat) => (
            <button
              key={cat}
              onClick={() => setCategory(cat === category ? null : cat)}
              className={`text-xs px-3 py-1.5 rounded-full transition-colors ${
                cat === category
                  ? 'bg-gray-900 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
              }`}
            >
              {cat}
            </button>
          ))}
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="h-6 w-6 border-2 border-gray-300 border-t-gray-600 rounded-full animate-spin" />
        </div>
      ) : filtered.length === 0 ? (
        <p className="text-center text-gray-500 py-12">No apps found.</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filtered.map((app) => (
            <AppCard key={app.name} app={app} installed={installed.has(app.name)} onInstall={handleInstall} />
          ))}
        </div>
      )}
    </div>
  );
}
