import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  fetchNotifications,
  markNotificationRead as apiMarkRead,
  markNotificationDismissed as apiMarkDismissed,
  markAllNotificationsRead as apiMarkAllRead,
  fetchUnreadCount,
} from '../api/client';
import type { Notification, NotificationsResponse } from '../types/dashboard';

export function useNotifications(state = 'all', page = 0, limit = 20) {
  return useQuery({
    queryKey: ['notifications', state, page],
    queryFn: () => fetchNotifications(state, limit, page * limit),
    staleTime: 30_000,
    refetchInterval: 15_000,
  });
}

export function useUnreadCount() {
  return useQuery({
    queryKey: ['notifications', 'unread-count'],
    queryFn: fetchUnreadCount,
    staleTime: 10_000,
    refetchInterval: 10_000,
  });
}

export function useNotificationMutations() {
  const queryClient = useQueryClient();

  function invalidateAll() {
    queryClient.invalidateQueries({ queryKey: ['notifications'] });
    queryClient.invalidateQueries({ queryKey: ['dashboard'] });
  }

  const markRead = useMutation({
    mutationFn: (id: string) => apiMarkRead(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const snapshots = new Map<string, NotificationsResponse | undefined>();
      let wasUnread = false;
      queryClient.getQueriesData<NotificationsResponse>({ queryKey: ['notifications'] }).forEach(([key, data]) => {
        if (!data) return;
        snapshots.set(JSON.stringify(key), data);
        const target = data.notifications.find((n: Notification) => n.id === id);
        if (target && !target.readAt) wasUnread = true;
        queryClient.setQueryData<NotificationsResponse>(key as string[], {
          ...data,
          notifications: data.notifications.map((n: Notification) =>
            n.id === id ? { ...n, readAt: new Date().toISOString() } : n
          ),
        });
      });
      // Decrement unread badge if the notification was previously unread
      if (wasUnread) {
        const prev = queryClient.getQueryData<{ count: number }>(['notifications', 'unread-count']);
        if (prev) {
          queryClient.setQueryData(['notifications', 'unread-count'], { count: Math.max(0, prev.count - 1) });
        }
      }
      return { snapshots };
    },
    onError: (_err, _id, ctx) => {
      ctx?.snapshots.forEach((data, keyStr) => {
        queryClient.setQueryData(JSON.parse(keyStr), data);
      });
    },
    onSettled: invalidateAll,
  });

  const markDismissed = useMutation({
    mutationFn: (id: string) => apiMarkDismissed(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const snapshots = new Map<string, NotificationsResponse | undefined>();
      let wasUnread = false;
      queryClient.getQueriesData<NotificationsResponse>({ queryKey: ['notifications'] }).forEach(([key, data]) => {
        if (!data) return;
        snapshots.set(JSON.stringify(key), data);
        const target = data.notifications.find((n: Notification) => n.id === id);
        if (target && !target.readAt) wasUnread = true;
        queryClient.setQueryData<NotificationsResponse>(key as string[], {
          ...data,
          notifications: data.notifications.filter((n: Notification) => n.id !== id),
          total: data.total - 1,
        });
      });
      // Decrement unread badge if the dismissed notification was unread
      if (wasUnread) {
        const prev = queryClient.getQueryData<{ count: number }>(['notifications', 'unread-count']);
        if (prev) {
          queryClient.setQueryData(['notifications', 'unread-count'], { count: Math.max(0, prev.count - 1) });
        }
      }
      return { snapshots };
    },
    onError: (_err, _id, ctx) => {
      ctx?.snapshots.forEach((data, keyStr) => {
        queryClient.setQueryData(JSON.parse(keyStr), data);
      });
    },
    onSettled: invalidateAll,
  });

  const markAllRead = useMutation({
    mutationFn: apiMarkAllRead,
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const snapshots = new Map<string, NotificationsResponse | undefined>();
      queryClient.getQueriesData<NotificationsResponse>({ queryKey: ['notifications'] }).forEach(([key, data]) => {
        if (!data) return;
        snapshots.set(JSON.stringify(key), data);
        queryClient.setQueryData<NotificationsResponse>(key as string[], {
          ...data,
          notifications: data.notifications.map((n: Notification) =>
            n.readAt ? n : { ...n, readAt: new Date().toISOString() }
          ),
        });
      });
      return { snapshots };
    },
    onError: (_err, _vars, ctx) => {
      ctx?.snapshots.forEach((data, keyStr) => {
        queryClient.setQueryData(JSON.parse(keyStr), data);
      });
    },
    onSettled: invalidateAll,
  });

  return { markRead, markDismissed, markAllRead };
}
