import { create } from 'zustand';
import type { SystemNoticePayload } from '../types/api';

export interface SystemNotice extends SystemNoticePayload {
  id: string;
}

interface NoticeStore {
  notices: SystemNotice[];
  addNotice: (notice: SystemNoticePayload) => void;
  dismissNotice: (id: string) => void;
  clearAll: () => void;
}

export const useNoticeStore = create<NoticeStore>((set) => ({
  notices: [],
  addNotice: (notice) =>
    set((state) => ({
      notices: [
        ...state.notices,
        { ...notice, id: `${Date.now()}-${Math.random().toString(36).slice(2)}` },
      ],
    })),
  dismissNotice: (id) =>
    set((state) => ({
      notices: state.notices.filter((n) => n.id !== id),
    })),
  clearAll: () => set({ notices: [] }),
}));
