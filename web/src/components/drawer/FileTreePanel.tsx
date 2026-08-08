import clsx from 'clsx';
import { useState } from 'react';
import type { FileNode } from '../../types';
import { useAppStore } from '../../store/useAppStore';

function getFileIcon(name: string): string {
  if (name.endsWith('.go')) return 'fa-solid fa-code text-cyan-400';
  if (name.endsWith('.md')) return 'fa-solid fa-file-lines text-amber-400';
  if (name.endsWith('.json')) return 'fa-solid fa-brackets-curly text-emerald-400';
  return 'fa-solid fa-file text-slate-400';
}

function FileTreeItem({ node, depth }: { node: FileNode; depth: number }) {
  const selectedFile = useAppStore((s) => s.selectedFile);
  const setSelectedFile = (file: FileNode) => useAppStore.setState({ selectedFile: file });
  const openContextMenu = useAppStore((s) => s.openContextMenu);
  const quoteFileToInput = useAppStore((s) => s.quoteFileToInput);

  const [expanded, setExpanded] = useState(node.expanded ?? false);

  if (node.type === 'folder') {
    return (
      <div className="space-y-1">
        <div
          onClick={() => setExpanded(!expanded)}
          className="flex items-center space-x-1.5 p-1 rounded hover:bg-slate-800/60 cursor-pointer text-slate-300"
        >
          <i className={clsx('fa-solid text-[10px] text-slate-500 w-3', expanded ? 'fa-chevron-down' : 'fa-chevron-right')}></i>
          <i className={clsx('fa-solid text-xs', depth === 0 ? 'fa-folder text-amber-500' : 'fa-folder-open text-amber-400')}></i>
          <span className={clsx(depth === 0 && 'font-bold')}>{node.name}</span>
        </div>
        {expanded && node.children && (
          <div className="pl-3 border-l border-slate-800/80 ml-2 space-y-1">
            {node.children.map((child) => (
              <FileTreeItem key={child.id} node={child} depth={depth + 1} />
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <div
      onContextMenu={(e) => { e.preventDefault(); openContextMenu(e.clientX, e.clientY, node); }}
      onClick={() => setSelectedFile(node)}
      className={clsx(
        'group flex items-center justify-between p-1 rounded cursor-pointer transition',
        selectedFile?.id === node.id
          ? 'bg-indigo-950/60 text-indigo-300 border border-indigo-500/30'
          : 'hover:bg-slate-900 text-slate-400'
      )}
    >
      <div className="flex items-center space-x-1.5 truncate">
        <i className={getFileIcon(node.name)}></i>
        <span className="truncate">{node.name}</span>
      </div>
      <button
        onClick={(e) => { e.stopPropagation(); quoteFileToInput(node); }}
        title="引用到对话"
        className="opacity-0 group-hover:opacity-100 px-1.5 py-0.5 rounded bg-indigo-600/80 hover:bg-indigo-500 text-white text-[9px] transition"
      >
        <i className="fa-solid fa-plus"></i>
      </button>
    </div>
  );
}

export function FileTreePanel() {
  const fileTree = useAppStore((s) => s.fileTree);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-slate-400">
        <span className="font-bold text-[11px] uppercase tracking-wider">Workspace Files</span>
        <span className="text-[10px] text-slate-500 font-mono">右键选择引用</span>
      </div>
      <div className="bg-slate-950 rounded-xl border border-slate-800 p-2 font-mono text-xs space-y-1 max-h-[calc(100vh-230px)] overflow-y-auto">
        {fileTree.map((item) => (
          <FileTreeItem key={item.id} node={item} depth={0} />
        ))}
      </div>
      <div className="text-[10px] text-slate-500 bg-slate-950/60 p-2 rounded-lg border border-slate-800/80">
        提示：在文件上右键弹出菜单引用，或悬停点击 + 将文件路径放入 Command Prompt。
      </div>
    </div>
  );
}
