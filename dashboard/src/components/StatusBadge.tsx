const statusConfig: Record<string, { dot: string; bg: string; text: string }> = {
  running: { dot: 'bg-emerald-500', bg: 'bg-emerald-50', text: 'text-emerald-700' },
  stopped: { dot: 'bg-gray-400', bg: 'bg-gray-50', text: 'text-gray-600' },
  installing: { dot: 'bg-amber-500', bg: 'bg-amber-50', text: 'text-amber-700' },
};

export default function StatusBadge({ status }: { status: string }) {
  const config = statusConfig[status] ?? statusConfig.stopped;
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full ${config.bg} ${config.text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${config.dot}`} />
      {status}
    </span>
  );
}
