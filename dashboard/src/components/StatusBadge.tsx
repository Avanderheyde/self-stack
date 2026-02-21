const statusConfig: Record<string, { dot: string; text: string }> = {
  running: { dot: 'bg-green-500', text: 'text-green-700' },
  stopped: { dot: 'bg-gray-400', text: 'text-gray-600' },
  installing: { dot: 'bg-yellow-500', text: 'text-yellow-700' },
};

export default function StatusBadge({ status }: { status: string }) {
  const config = statusConfig[status] ?? statusConfig.stopped;
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${config.text}`}>
      <span className={`w-2 h-2 rounded-full ${config.dot}`} />
      {status}
    </span>
  );
}
