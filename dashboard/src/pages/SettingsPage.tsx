export default function SettingsPage() {
  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900 mb-6">Settings</h1>

      <div className="space-y-6">
        <div className="border border-gray-200 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-2">Network & Tunnels</h2>
          <p className="text-sm text-gray-400">
            Tunnel configuration and remote access settings will appear here in a future update.
          </p>
        </div>

        <div className="border border-gray-200 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-2">Devices</h2>
          <p className="text-sm text-gray-400">
            Connected devices and pairing settings will appear here in a future update.
          </p>
        </div>

        <div className="border border-gray-200 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-2">About</h2>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-500">Version</span>
              <span className="text-gray-900 font-medium">0.1.0</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">Project</span>
              <span className="text-gray-900 font-medium">SelfStack</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
