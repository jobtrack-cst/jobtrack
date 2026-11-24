import { isNotFound } from '../application-status/shared';
import { listApplications, type Application } from '../application/api';
import { allPages } from '../application/applicant';
import { listMessages, type Message } from './api';

// One inbox row: an application and its last message. `last` is null when the thread
// has no messages; `failed` means the last message could not be read.
export interface InboxItem { app: Application; last: Message | null; failed: boolean }

// Messages are listed oldest first, so the first page gives totalCount, and a thread
// longer than one page is read again at offset totalCount - 1, which returns only the
// last message. NOT_FOUND means messaging has not processed the application yet: no
// messages, not an error.
export async function lastMessage(applicationId: string): Promise<Message | null> {
  try {
    const first = await listMessages(applicationId, '');
    const items = first.messages ?? [];
    const total = Number(first.pagination?.totalCount ?? items.length);
    if (total <= items.length || !first.pagination?.nextPageToken) return items[items.length - 1] ?? null;
    const tail = (await listMessages(applicationId, String(total - 1))).messages ?? [];
    return tail[tail.length - 1] ?? items[items.length - 1] ?? null;
  } catch (error) {
    if (isNotFound(error)) return null;
    throw error;
  }
}

const time = (value?: string) => Date.parse(value ?? '') || 0;

// Newest activity first: the last message's time, or the applied date when there is no
// message. Ties by applied date, then by application ID, so the order is stable.
export function activityOf(item: InboxItem) {
  return time(item.last?.timestamps?.createdAt) || time(item.app.timestamps?.createdAt);
}
export function sortInbox(items: InboxItem[]) {
  return [...items].sort((a, b) => activityOf(b) - activityOf(a)
    || time(b.app.timestamps?.createdAt) - time(a.app.timestamps?.createdAt)
    || a.app.applicationId.localeCompare(b.app.applicationId));
}

// Every application of the caller; each one is a conversation. Read first and on its
// own, so an open thread does not wait for every row's last message.
export const loadApplications = () =>
  allPages(token => listApplications(token).then(r => ({ items: r.applications, pagination: r.pagination })));

// The applications with their last messages, sorted for the list. A failed message read
// marks only that row, never the whole inbox.
export async function withLastMessages(apps: Application[]): Promise<InboxItem[]> {
  const items = await Promise.all(apps.map(async (app): Promise<InboxItem> => {
    try { return { app, last: await lastMessage(app.applicationId), failed: false }; }
    catch { return { app, last: null, failed: true }; }
  }));
  return sortInbox(items);
}

// The row's time: the clock time today, otherwise the short date.
export function rowTime(value?: string, now = new Date()) {
  const date = value ? new Date(value) : undefined;
  if (!date || Number.isNaN(date.getTime())) return '';
  if (date.toDateString() === now.toDateString()) return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: date.getFullYear() === now.getFullYear() ? undefined : 'numeric' });
}

// One line of the last message, with line breaks and runs of spaces collapsed.
export const previewOf = (content: string) => content.replace(/\s+/g, ' ').trim();
