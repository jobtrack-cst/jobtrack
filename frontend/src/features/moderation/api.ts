import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';

export type ReportStatus = 'REPORT_STATUS_UNSPECIFIED' | 'REPORT_STATUS_OPEN' | 'REPORT_STATUS_DISMISSED' | 'REPORT_STATUS_ACTIONED';
export interface Report {
  reportId: string;
  jobId: string;
  reporterId?: string;
  reason?: string;
  status?: ReportStatus;
  timestamps?: { createdAt?: string; updatedAt?: string };
}

export const reportJob = (jobId: string, reason: string) =>
  api<{ report: Report }>(`/v1/jobs/${idPath(jobId)}/reports`, { method: 'POST', body: { reason } });
// No status lists every status.
export const listReports = (token: string, status?: ReportStatus) =>
  api<{ reports?: Report[]; pagination?: Pagination }>(`/v1/moderation/reports?${pageQuery(token)}${status ? `&status=${status}` : ''}`);
export const dismissReport = (reportId: string) =>
  api<{ report: Report }>(`/v1/moderation/reports/${idPath(reportId)}/dismiss`, { method: 'POST', body: {} });
export const removeJob = (jobId: string, reason: string) =>
  api<{ jobId?: string }>(`/v1/moderation/jobs/${idPath(jobId)}/remove`, { method: 'POST', body: { reason } });
