import type { ReactNode } from 'react';
import { useFocusTrap } from '../common/useFocusTrap';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  titleIcon: string;
  children: ReactNode;
  onSubmit: () => void;
  submitLabel: string;
}

export function Modal({ isOpen, onClose, title, titleIcon, children, onSubmit, submitLabel }: ModalProps) {
  const containerRef = useFocusTrap<HTMLDivElement>({
    isActive: isOpen,
    onClose,
  });

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div ref={containerRef} className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 space-y-4 shadow-2xl" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
            <i className={`${titleIcon} text-amber-400`}></i>
            <span>{title}</span>
          </h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>
        {children}
        <div className="flex justify-end space-x-2 pt-2 border-t border-slate-800">
          <button onClick={onClose} className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium">取消</button>
          <button onClick={onSubmit} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-lg shadow-amber-600/20">{submitLabel}</button>
        </div>
      </div>
    </div>
  );
}
