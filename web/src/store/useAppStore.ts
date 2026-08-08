import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type {
  PrimaryNavType,
  SettingsTabType,
  DrawerTabType,
  ThreadFilterType,
  MemberFilterType,
  FileNode,
  ContextMenuState,
  Notification,
  Toast,
} from '../types';
import { apiGet, apiPatch, apiDelete } from '../services/http';

interface AppStore {
  // Navigation state (UI only)
  activeNav: PrimaryNavType;
  activeSettingsTab: SettingsTabType;
  activeThreadId: string;
  showRightDrawer: boolean;
  leftPanelVisible: boolean;
  middlePanelVisible: boolean;
  rightPanelVisible: boolean;
  activeDrawerTab: DrawerTabType;

  // Thread UI state (filters only — data in useThreads hook)
  threadSearchInput: string;
  threadFilter: ThreadFilterType;

  // Settings UI state (filters only — data in useSettings hook)
  memberFilter: MemberFilterType;
  globalDefaultDog: string;

  // Interaction state
  userPromptInput: string;
  contextMenu: ContextMenuState;
  selectedFile: FileNode | null;

  // Modal state
  showAddMemberModal: boolean;
  showAddAccountModal: boolean;

  // Notification state (UI notification center)
  notifications: Notification[];

  // File tree (static mock for now — no file API)
  fileTree: FileNode[];

  // Toast state (global UI)
  toasts: Toast[];

  // Mention state (P0: manual only, popover not shown)
  mentionOpen: boolean;
  mentionQuery: string;

  // Actions — Navigation
  setActiveNav: (nav: PrimaryNavType) => void;
  setActiveSettingsTab: (tab: SettingsTabType) => void;
  setActiveDrawerTab: (tab: DrawerTabType) => void;
  toggleRightDrawer: () => void;
  toggleLeftPanel: () => void;
  toggleMiddlePanel: () => void;
  toggleRightPanel: () => void;

  // Actions — Thread UI
  switchThread: (id: string) => void;
  setThreadSearchInput: (input: string) => void;
  setThreadFilter: (filter: ThreadFilterType) => void;

  // Actions — Settings UI
  setMemberFilter: (filter: MemberFilterType) => void;
  setGlobalDefaultDog: (dog: string) => void;

  // Actions — Interaction
  setUserPromptInput: (input: string) => void;

  // Actions — Context Menu
  openContextMenu: (x: number, y: number, file: FileNode) => void;
  closeContextMenu: () => void;
  quoteFileToInput: (file: FileNode | null) => void;
  quoteAndSendFile: (file: FileNode | null) => void;

  // Actions — Modals
  setShowAddMemberModal: (show: boolean) => void;
  setShowAddAccountModal: (show: boolean) => void;

  // Actions — Notifications
  fetchNotifications: () => Promise<void>;
  markNotificationRead: (id: string) => void;
  markAllNotificationsRead: () => void;
  clearNotifications: () => void;

  // Actions — File Tree
  fetchFileTree: () => Promise<void>;

  // Actions — Toast
  showToast: (toast: Omit<Toast, 'id'>) => void;
  dismissToast: (id: string) => void;

  // Actions — Mention
  setMentionOpen: (open: boolean) => void;
  setMentionQuery: (query: string) => void;
}

export const useAppStore = create<AppStore>()(
  persist(
    (set) => ({
      activeNav: 'threads',
      activeSettingsTab: 'members',
      activeThreadId: 'thread-1',
      showRightDrawer: true,
      leftPanelVisible: true,
      middlePanelVisible: true,
      rightPanelVisible: true,
      activeDrawerTab: 'plan',

      threadSearchInput: '',
      threadFilter: 'all',

      memberFilter: 'all',
      globalDefaultDog: '',

      userPromptInput: '',
      contextMenu: { show: false, x: 0, y: 0, file: null },
      selectedFile: null,

      showAddMemberModal: false,
      showAddAccountModal: false,

      notifications: [],
      fileTree: [],
      toasts: [],

      mentionOpen: false,
      mentionQuery: '',

      setActiveNav: (nav) => set({ activeNav: nav }),
      setActiveSettingsTab: (tab) => set({ activeSettingsTab: tab }),
      setActiveDrawerTab: (tab) => set({ activeDrawerTab: tab }),
      toggleRightDrawer: () => set((state) => ({ showRightDrawer: !state.showRightDrawer })),
      toggleLeftPanel: () =>
        set((state) => {
          if (state.leftPanelVisible && !state.middlePanelVisible && !state.rightPanelVisible) return state;
          return { leftPanelVisible: !state.leftPanelVisible };
        }),
      toggleMiddlePanel: () =>
        set((state) => {
          if (state.middlePanelVisible && !state.leftPanelVisible && !state.rightPanelVisible) return state;
          return { middlePanelVisible: !state.middlePanelVisible };
        }),
      toggleRightPanel: () =>
        set((state) => {
          if (state.rightPanelVisible && !state.leftPanelVisible && !state.middlePanelVisible) return state;
          return { rightPanelVisible: !state.rightPanelVisible };
        }),

      switchThread: (id) => set({ activeThreadId: id }),
      setThreadSearchInput: (input) => set({ threadSearchInput: input }),
      setThreadFilter: (filter) => set({ threadFilter: filter }),

      setMemberFilter: (filter) => set({ memberFilter: filter }),
      setGlobalDefaultDog: (dog) => set({ globalDefaultDog: dog }),

      setUserPromptInput: (input) => set({ userPromptInput: input }),

      openContextMenu: (x, y, file) =>
        set({ contextMenu: { show: true, x, y, file } }),
      closeContextMenu: () =>
        set((state) => ({ contextMenu: { ...state.contextMenu, show: false } })),
      quoteFileToInput: (file) => {
        if (!file || !file.path) return;
        set((state) => ({
          userPromptInput: `${state.userPromptInput} @${file.path} `,
          contextMenu: { ...state.contextMenu, show: false },
        }));
      },
      quoteAndSendFile: (file) => {
        if (!file || !file.path) return;
        set((state) => ({
          userPromptInput: `${state.userPromptInput} @${file.path} `,
          contextMenu: { ...state.contextMenu, show: false },
        }));
      },

      setShowAddMemberModal: (show) => set({ showAddMemberModal: show }),
      setShowAddAccountModal: (show) => set({ showAddAccountModal: show }),

      fetchNotifications: async () => {
        try {
          const data = await apiGet<Notification[]>('/api/notifications');
          set({ notifications: Array.isArray(data) ? data : [] });
        } catch { /* graceful: keep empty */ }
      },
      markNotificationRead: (id) => {
        apiPatch(`/api/notifications/${id}/read`, {}).catch(() => {});
        set((state) => ({
          notifications: state.notifications.map((n) =>
            n.id === id ? { ...n, read: true } : n
          ),
        }));
      },
      markAllNotificationsRead: () =>
        set((state) => ({
          notifications: state.notifications.map((n) => ({ ...n, read: true })),
        })),
      clearNotifications: () => {
        apiDelete('/api/notifications').catch(() => {});
        set({ notifications: [] });
      },

      fetchFileTree: async () => {
        try {
          const data = await apiGet<FileNode[]>('/api/files/tree');
          set({ fileTree: Array.isArray(data) ? data : [] });
        } catch { /* graceful: keep empty */ }
      },

      showToast: (toast) =>
        set((state) => ({
          toasts: [
            ...state.toasts,
            { ...toast, id: `toast-${Date.now()}-${Math.random().toString(36).slice(2)}` },
          ],
        })),
      dismissToast: (id) =>
        set((state) => ({
          toasts: state.toasts.filter((t) => t.id !== id),
        })),

      setMentionOpen: (open) => set({ mentionOpen: open }),
      setMentionQuery: (query) => set({ mentionQuery: query }),
    }),
    {
      name: 'soloweb-panel-state',
      partialize: (state): Pick<AppStore, 'leftPanelVisible' | 'middlePanelVisible' | 'rightPanelVisible'> => ({
        leftPanelVisible: state.leftPanelVisible,
        middlePanelVisible: state.middlePanelVisible,
        rightPanelVisible: state.rightPanelVisible,
      }),
    }
  )
);
