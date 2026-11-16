import { api } from '../../api/client';
import { pageQuery, type Pagination } from '../application/shared';

export interface AuditEntry { eventId: string; eventType?: string; producer?: string; occurredAt?: string; dataJson?: string }
// Empty values are not sent. Times are RFC 3339 in UTC; start inclusive, end exclusive (JT-35).
export interface AuditFilters { eventType: string; startTime?: string; endTime?: string }

export function listAuditEntries(token: string, filters: AuditFilters) {
  let query = pageQuery(token);
  if (filters.eventType) query += `&event_type=${encodeURIComponent(filters.eventType)}`;
  if (filters.startTime) query += `&start_time=${encodeURIComponent(filters.startTime)}`;
  if (filters.endTime) query += `&end_time=${encodeURIComponent(filters.endTime)}`;
  return api<{ entries?: AuditEntry[]; pagination?: Pagination }>(`/v1/audit/entries?${query}`);
}
