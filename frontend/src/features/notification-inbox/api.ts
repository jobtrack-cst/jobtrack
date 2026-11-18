import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';
export interface Notification { notificationId: string; title: string; body: string; read?: boolean; createdAt: string }
export const listNotifications = (token: string, unread: boolean) => api<{ notifications?: Notification[]; pagination?: Pagination }>(`/v1/notifications?${pageQuery(token)}&unread_only=${unread}`);
export const markRead = (id: string) => api<{ notification: Notification }>(`/v1/notifications/${idPath(id)}/read`, { method: 'POST', body: {} });
