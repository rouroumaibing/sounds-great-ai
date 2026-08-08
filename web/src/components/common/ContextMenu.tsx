import { createPortal } from 'react-dom';
import { useEffect } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useFocusTrap } from './useFocusTrap';

export function ContextMenu() {
  const contextMenu = useAppStore((s) => s.contextMenu);
  const closeContextMenu = useAppStore((s) => s.closeContextMenu);
  const quoteFileToInput = useAppStore((s) => s.quoteFileToInput);
  const quoteAndSendFile = useAppStore((s) => s.quoteAndSendFile);

  const containerRef = useFocusTrap<HTMLDivElement>({
    isActive: contextMenu.show,
    onClose: closeContextMenu,
  });

  useEffect(() => {
    if (!contextMenu.show) return;
    const handleMouseDown = (e: MouseEvent) => {
      const container = containerRef.current;
      if (container && !container.contains(e.target as Node)) {
        closeContextMenu();
      }
    };
    document.addEventListener('mousedown', handleMouseDown);
    return () => document.removeEventListener('mousedown', handleMouseDown);
  }, [contextMenu.show, closeContextMenu, containerRef]);

  if (!contextMenu.show) return null;

  return createPortal(
    <div
      ref={containerRef}
      style={{ top: `${contextMenu.y}px`, left: `${contextMenu.x}px` }}
      className="fixed z-50 bg-slate-900/95 border border-slate-700 backdrop-blur-md rounded-xl shadow-2xl py-1 w-56 text-xs text-slate-200"
      role="menu"
      aria-label="文件操作菜单"
    >
      <div className="px-3 py-1.5 font-bold border-b border-slate-800 text-[10px] text-indigo-300 font-mono truncate flex items-center space-x-1.5">
        <i className="fa-solid fa-file-code"></i>
        <span className="truncate">{contextMenu.file?.path}</span>
      </div>
      <button onClick={() => quoteFileToInput(contextMenu.file)} className="w-full text-left px-3 py-2 hover:bg-indigo-600 hover:text-white flex items-center space-x-2 transition">
        <i className="fa-solid fa-at text-indigo-400"></i>
        <span>引用到对话框 (@)</span>
      </button>
      <button onClick={() => quoteAndSendFile(contextMenu.file)} className="w-full text-left px-3 py-2 hover:bg-emerald-600 hover:text-white flex items-center space-x-2 transition">
        <i className="fa-solid fa-paper-plane text-emerald-400"></i>
        <span>直接下发至 Task 审查</span>
      </button>
      <div className="border-t border-slate-800 my-1"></div>
      <button onClick={closeContextMenu} className="w-full text-left px-3 py-1.5 hover:bg-slate-800 text-slate-400 text-[11px]">
        <i className="fa-solid fa-xmark mr-1"></i> 关闭
      </button>
    </div>,
    document.body
  );
}
