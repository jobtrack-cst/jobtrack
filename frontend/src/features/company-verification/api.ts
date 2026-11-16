import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';

export type RequestStatus = 'REQUEST_STATUS_UNSPECIFIED' | 'REQUEST_STATUS_PENDING' | 'REQUEST_STATUS_APPROVED' | 'REQUEST_STATUS_REJECTED';
export interface VerificationRequest {
  requestId: string;
  employerId: string;
  registrationNumber?: string;
  notes?: string;
  status?: RequestStatus;
  rejectionReason?: string;
  reviewedBy?: string;
  timestamps?: { createdAt?: string; updatedAt?: string };
}
export interface RequestPage { requests?: VerificationRequest[]; pagination?: Pagination }
type OneRequest = { request: VerificationRequest };

export const submitVerificationRequest = (registrationNumber: string, notes: string) =>
  api<OneRequest>('/v1/verification/requests', { method: 'POST', body: { registrationNumber, notes } });
export const listMyVerificationRequests = (token: string) =>
  api<RequestPage>(`/v1/verification/my-requests?${pageQuery(token)}`);
// No status lists every status.
export const listVerificationRequests = (token: string, status?: RequestStatus) =>
  api<RequestPage>(`/v1/verification/requests?${pageQuery(token)}${status ? `&status=${status}` : ''}`);
export const approveVerificationRequest = (requestId: string) =>
  api<OneRequest>(`/v1/verification/requests/${idPath(requestId)}/approve`, { method: 'POST', body: {} });
export const rejectVerificationRequest = (requestId: string, reason: string) =>
  api<OneRequest>(`/v1/verification/requests/${idPath(requestId)}/reject`, { method: 'POST', body: { reason } });
