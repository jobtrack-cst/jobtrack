import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';

export type ApplicationStatus = 'APPLICATION_STATUS_UNSPECIFIED' | 'APPLICATION_STATUS_SUBMITTED' | 'APPLICATION_STATUS_UNDER_REVIEW'
  | 'APPLICATION_STATUS_SHORTLISTED' | 'APPLICATION_STATUS_INTERVIEW' | 'APPLICATION_STATUS_OFFERED' | 'APPLICATION_STATUS_ACCEPTED'
  | 'APPLICATION_STATUS_DECLINED' | 'APPLICATION_STATUS_REJECTED' | 'APPLICATION_STATUS_WITHDRAWN';
export interface StatusRecord { applicationId: string; status: ApplicationStatus; timestamps?: { createdAt?: string; updatedAt?: string } }
export interface HistoryEntry { previousStatus: ApplicationStatus; newStatus: ApplicationStatus; occurredAt?: string }

export const getStatus = (applicationId: string) => api<{ statusRecord: StatusRecord }>(`/v1/applications/${idPath(applicationId)}/status`);
export const listHistory = (applicationId: string, token: string) =>
  api<{ entries?: HistoryEntry[]; pagination?: Pagination }>(`/v1/applications/${idPath(applicationId)}/status-history?${pageQuery(token)}`);
