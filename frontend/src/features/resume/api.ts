import { api } from '../../api/client';
import { pageQuery, type Pagination } from '../application/shared';

export type ValidationStatus = 'RESUME_VALIDATION_STATUS_UNSPECIFIED' | 'RESUME_VALIDATION_STATUS_PENDING' | 'RESUME_VALIDATION_STATUS_VALIDATED' | 'RESUME_VALIDATION_STATUS_REJECTED';
export interface Resume {
  resumeId: string; fileName: string; contentType?: string; sizeBytes?: string | number;
  validationStatus: ValidationStatus; rejectionReason?: string; timestamps?: { createdAt?: string; updatedAt?: string };
}
export const listMyResumes = (token: string) => api<{ resumes?: Resume[]; pagination?: Pagination }>(`/v1/resumes?${pageQuery(token)}`);
// content is the file as base64, which is how grpc-gateway carries proto bytes in JSON.
export const uploadResume = (fileName: string, content: string) =>
  api<{ resume: Resume }>('/v1/resumes', { method: 'POST', body: { fileName, contentType: 'application/pdf', content } });
