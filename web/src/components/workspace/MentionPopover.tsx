import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useBreeds } from '../../hooks/useBreeds';

interface MentionPopoverProps {
  onSelect: (insertText: string) => void;
}

interface MentionItem {
  type: 'file' | 'folder' | 'agent';
  label: string;
  insertText: string;
}

export function MentionPopover({ onSelect }: MentionPopoverProps) {
  const mentionQuery = useAppStore((s) => s.mentionQuery);
  const setMentionOpen = useAppStore((s) => s.setMentionOpen);
  const setMentionQuery = useAppStore((s) => s.setMentionQuery);
  const fileTree = useAppStore((s) => s.fileTree);
  const { dogs } = useBreeds();

  const [selectedIndex, setSelectedIndex] = useState(0);

  const allItems: MentionItem[] = [
    ...fileTree.filter((n) => n.type === 'folder').map((n) => ({
      type: 'folder' as const,
      label: n.name,
      insertText: `@${n.path ?? n.name}`,
    })),
    ...fileTree.filter((n) => n.type === 'file').map((n) => ({
      type: 'file' as const,
      label: n.name,
      insertText: `@${n.path ?? n.name}`,
    })),
    ...dogs.map((d) => ({
      type: 'agent' as const,
      label: d.name,
      insertText: `@${d.id}`,
    })),
  ];

  const filteredItems = mentionQuery
    ? allItems.filter((item) => item.label.toLowerCase().includes(mentionQuery.toLowerCase()))
    : allItems;

  const safeIndex = Math.min(selectedIndex, filteredItems.length - 1);

  useEffect(() => {
    setSelectedIndex(0);
  }, [mentionQuery]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIndex((prev) => Math.min(prev + 1, filteredItems.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIndex((prev) => Math.max(prev - 1, 0));
      } else if (e.key === 'Enter') {
        e.preventDefault();
        const item = filteredItems[safeIndex];
        if (item) {
          onSelect(item.insertText);
          setMentionOpen(false);
          setMentionQuery('');
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        setMentionOpen(false);
        setMentionQuery('');
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [filteredItems, safeIndex, onSelect, setMentionOpen, setMentionQuery]);

  if (filteredItems.length === 0) {
    return (
      <div className="absolute bottom-full mb-2 left-4 w-72 max-h-48 overflow-y-auto bg-slate-900 border border-slate-700 rounded-lg shadow-xl z-50 p-3 text-xs text-slate-500">
        No matches found
      </div>
    );
  }

  return (
    <div className="absolute bottom-full mb-2 left-4 w-72 max-h-48 overflow-y-auto bg-slate-900 border border-slate-700 rounded-lg shadow-xl z-50">
      {filteredItems.map((item, idx) => (
        <button
          key={idx}
          onClick={() => {
            onSelect(item.insertText);
            setMentionOpen(false);
            setMentionQuery('');
          }}
          className={clsx(
            'w-full px-3 py-1.5 text-left text-xs flex items-center gap-2 transition',
            idx === safeIndex ? 'bg-indigo-500/20 text-indigo-300' : 'text-slate-300 hover:bg-slate-800'
          )}
        >
          <i className={clsx('fa-solid text-[10px]', {
            'fa-folder': item.type === 'folder',
            'fa-file': item.type === 'file',
            'fa-dog': item.type === 'agent',
          })}></i>
          <span className="truncate">{item.label}</span>
        </button>
      ))}
    </div>
  );
}
