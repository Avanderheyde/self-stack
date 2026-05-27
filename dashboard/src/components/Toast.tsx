import { useEffect, useState } from 'react';

interface ToastMessage {
  id: number;
  text: string;
  type: 'error' | 'success';
}

let nextId = 0;
let addToast: (text: string, type: 'error' | 'success') => void = () => {};

export function toast(text: string, type: 'error' | 'success' = 'error') {
  addToast(text, type);
}

export default function ToastContainer() {
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  useEffect(() => {
    addToast = (text, type) => {
      const id = nextId++;
      setToasts((prev) => [...prev, { id, text, type }]);
      setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000);
    };
    return () => { addToast = () => {}; };
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-20 sm:bottom-6 left-4 right-4 sm:left-auto sm:right-6 sm:w-auto flex flex-col gap-2 z-[60]">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`px-4 py-3 rounded-2xl shadow-lg text-sm font-medium animate-slide-up backdrop-blur-sm ${
            t.type === 'error'
              ? 'bg-red-500/95 text-white'
              : 'bg-emerald-500/95 text-white'
          }`}
        >
          {t.text}
        </div>
      ))}
    </div>
  );
}
