import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';

export interface Interview { interviewId: string; applicationId: string; scheduledAt?: string; details?: string }

export const listInterviews = (applicationId: string, token: string) =>
  api<{ interviews?: Interview[]; pagination?: Pagination }>(`/v1/applications/${idPath(applicationId)}/interviews?${pageQuery(token)}`);
export const scheduleInterview = (applicationId: string, scheduledAt: string, details: string) =>
  api<{ interview: Interview }>(`/v1/applications/${idPath(applicationId)}/interviews`, { method: 'POST', body: { scheduledAt, details } });
