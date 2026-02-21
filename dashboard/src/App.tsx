import { Routes, Route } from 'react-router';
import Layout from './components/Layout';
import ToastContainer from './components/Toast';
import StorePage from './pages/StorePage';
import MyAppsPage from './pages/MyAppsPage';
import AppDetailPage from './pages/AppDetailPage';
import SettingsPage from './pages/SettingsPage';

export default function App() {
  return (
    <>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<StorePage />} />
          <Route path="my-apps" element={<MyAppsPage />} />
          <Route path="apps/:name" element={<AppDetailPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
      <ToastContainer />
    </>
  );
}
