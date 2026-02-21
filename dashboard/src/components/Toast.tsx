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
    <div className="fixed bottom-6 right-6 flex flex-col gap-2 z-50">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`px-4 py-3 rounded-xl shadow-lg text-sm font-medium animate-fade-in ${
            t.type === 'error'
              ? 'bg-red-600 text-white'
              : 'bg-green-600 text-white'
          }`}
        >
          {t.text}
        </div>
      ))}
    </div>
  );
}
