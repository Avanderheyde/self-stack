import { useEffect, useState } from 'react';

export default function SearchBar({ onSearch }: { onSearch: (q: string) => void }) {
  const [value, setValue] = useState('');

  useEffect(() => {
    const t = setTimeout(() => onSearch(value), 300);
    return () => clearTimeout(t);
  }, [value, onSearch]);

  return (
    <div className="relative">
      <svg
        className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>
      <input
        type="text"
        placeholder="Search apps..."
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="w-full pl-10 pr-4 py-2.5 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-gray-200 focus:border-gray-300 transition-colors"
      />
    </div>
  );
}
