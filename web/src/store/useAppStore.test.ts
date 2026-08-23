import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// useAppStore dynamically imports useChatStore inside quoteAndSendFile; mock
// it so the test asserts the hand-off without a real WS stack.
const sendPrompt = vi.fn();
vi.mock('./useChatStore', () => ({
  useChatStore: { getState: () => ({ sendPrompt }) },
}));

import { useAppStore } from './useAppStore';

beforeEach(() => {
  sendPrompt.mockClear();
  useAppStore.setState({
    userPromptInput: '',
    notifications: [],
    toasts: [],
    contextMenu: { show: true, x: 1, y: 1, file: { id: 'f', name: 'a.ts', type: 'file', path: 'src/a.ts' } },
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('useAppStore notifications', () => {
  it('fetchNotifications fills the store from /api/notifications', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify([{ id: 'n1', severity: 'info', title: 'T', message: 'M', source: 'system', timestamp: '12:00:00', read: false }]), { status: 200 })
    ));
    await useAppStore.getState().fetchNotifications();
    expect(useAppStore.getState().notifications).toHaveLength(1);
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining('/api/notifications'), expect.anything());
  });

  it('fetchNotifications keeps empty list on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    await useAppStore.getState().fetchNotifications();
    expect(useAppStore.getState().notifications).toEqual([]);
  });

  it('pushLiveNotice prepends an unread notification with a generated id', () => {
    useAppStore.getState().pushLiveNotice({ severity: 'warning', title: 't', message: 'm', source: 'system', timestamp: 'x' });
    const list = useAppStore.getState().notifications;
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({ title: 't', read: false });
    expect(list[0].id).toBeTruthy();
  });

  it('markNotificationRead PATCHes and flips local read state', () => {
    useAppStore.setState({ notifications: [{ id: 'n1', severity: 'info', title: 'a', message: 'b', source: 's', timestamp: 't', read: false }] });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 })));
    useAppStore.getState().markNotificationRead('n1');
    expect(useAppStore.getState().notifications[0].read).toBe(true);
    const [url, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/api/notifications/n1/read');
    expect(init.method).toBe('PATCH');
  });

  it('clearNotifications issues DELETE and empties the list', () => {
    useAppStore.setState({ notifications: [{ id: 'n1', severity: 'info', title: 'a', message: 'b', source: 's', timestamp: 't', read: true }] });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 })));
    useAppStore.getState().clearNotifications();
    expect(useAppStore.getState().notifications).toEqual([]);
    const [url, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/api/notifications');
    expect(init.method).toBe('DELETE');
  });
});

describe('useAppStore context menu', () => {
  it('quoteFileToInput appends @path and closes the menu', () => {
    useAppStore.getState().quoteFileToInput({ id: 'f', name: 'a.ts', type: 'file', path: 'src/a.ts' });
    const s = useAppStore.getState();
    expect(s.userPromptInput).toBe(' @src/a.ts ');
    expect(s.contextMenu.show).toBe(false);
  });

  it('quoteAndSendFile quotes AND immediately hands off to sendPrompt', async () => {
    useAppStore.getState().quoteAndSendFile({ id: 'f', name: 'a.ts', type: 'file', path: 'src/a.ts' });
    const s = useAppStore.getState();
    expect(s.userPromptInput).toBe(' @src/a.ts ');
    expect(s.contextMenu.show).toBe(false);
    // The chat store is imported dynamically inside the action.
    await vi.waitFor(() => expect(sendPrompt).toHaveBeenCalledTimes(1));
  });
});

describe('useAppStore toasts', () => {
  it('showToast assigns a unique id; dismissToast removes it', () => {
    useAppStore.getState().showToast({ message: 'm1', type: 'info' });
    useAppStore.getState().showToast({ message: 'm2', type: 'error' });
    const first = useAppStore.getState().toasts;
    expect(first).toHaveLength(2);
    expect(first[0].id).not.toBe(first[1].id);
    useAppStore.getState().dismissToast(first[0].id);
    expect(useAppStore.getState().toasts.map((t) => t.message)).toEqual(['m2']);
  });
});
