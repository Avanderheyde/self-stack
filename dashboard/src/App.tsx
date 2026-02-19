import { useEffect, useState } from 'react';
import { getApps, searchRegistry, installApp, startApp, stopApp, removeApp } from './api';
import type { App, RegistryApp } from './api';

function AppCard({ app, onAction }: { app: App; onAction: () => void }) {
  return (
    <div style={{ border: '1px solid #ddd', borderRadius: 8, padding: 16, margin: 8 }}>
      <h3>{app.DisplayName || app.Name}</h3>
      <p>{app.Description}</p>
      <span style={{ 
        color: app.Status === 'running' ? 'green' : 'gray',
        fontWeight: 'bold'
      }}>{app.Status}</span>
      <span style={{ marginLeft: 8, color: '#666' }}>port {app.HostPort}</span>
      <div style={{ marginTop: 8 }}>
        {app.Status === 'stopped' && <button onClick={() => startApp(app.Name).then(onAction)}>Start</button>}
        {app.Status === 'running' && <button onClick={() => stopApp(app.Name).then(onAction)}>Stop</button>}
        <button onClick={() => { if(confirm(`Remove ${app.Name}?`)) removeApp(app.Name).then(onAction) }} style={{ marginLeft: 8, color: 'red' }}>Remove</button>
      </div>
    </div>
  );
}

function RegistryCard({ app, onInstall }: { app: RegistryApp; onInstall: () => void }) {
  return (
    <div style={{ border: '1px solid #ddd', borderRadius: 8, padding: 16, margin: 8 }}>
      <h3>{app.display_name}</h3>
      <p>{app.description}</p>
      <span style={{ color: '#666' }}>{app.category}</span>
      {app.verified && <span style={{ marginLeft: 8, color: 'green' }}>Verified</span>}
      <div style={{ marginTop: 8 }}>
        <button onClick={() => installApp(app.name).then(onInstall)}>Install</button>
      </div>
    </div>
  );
}

export default function Dashboard() {
  const [apps, setApps] = useState<App[]>([]);
  const [registry, setRegistry] = useState<RegistryApp[]>([]);
  const [tab, setTab] = useState<'installed' | 'store'>('installed');
  const [search, setSearch] = useState('');

  const refresh = () => { getApps().then(setApps); };
  useEffect(() => { refresh(); }, []);
  useEffect(() => {
    if (tab === 'store') searchRegistry(search).then(setRegistry);
  }, [tab, search]);

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: 20, fontFamily: 'system-ui' }}>
      <header style={{ marginBottom: 20 }}>
        <h1>SelfStack</h1>
      </header>
      <nav style={{ marginBottom: 20 }}>
        <button 
          onClick={() => setTab('installed')} 
          style={{ fontWeight: tab === 'installed' ? 'bold' : 'normal', marginRight: 8 }}
        >My Apps</button>
        <button 
          onClick={() => setTab('store')} 
          style={{ fontWeight: tab === 'store' ? 'bold' : 'normal' }}
        >App Store</button>
      </nav>
      <main>
        {tab === 'installed' && (
          <div>
            {apps.length === 0 && <p>No apps installed. Visit the App Store to get started.</p>}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 8 }}>
              {apps.map(a => <AppCard key={a.Name} app={a} onAction={refresh} />)}
            </div>
          </div>
        )}
        {tab === 'store' && (
          <div>
            <input 
              type="text" 
              placeholder="Search apps..." 
              value={search} 
              onChange={e => setSearch(e.target.value)}
              style={{ padding: 8, marginBottom: 16, width: '100%', boxSizing: 'border-box' }}
            />
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 8 }}>
              {registry.map(a => <RegistryCard key={a.name} app={a} onInstall={refresh} />)}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
