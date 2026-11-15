import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';
export interface Message { messageId: string; senderId: string; content: string; timestamps?: { createdAt?: string } }
export const listMessages = (id: string, token: string) => api<{ messages?: Message[]; pagination?: Pagination }>(`/v1/applications/${idPath(id)}/messages?${pageQuery(token)}`);
export const sendMessage = (id: string, content: string) => api<{ message: Message }>(`/v1/applications/${idPath(id)}/messages`, { method: 'POST', body: { content } });
