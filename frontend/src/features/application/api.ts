import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from './shared';
export interface Application { applicationId: string; jobId: string; applicantId: string; employerId: string; resumeId: string; withdrawn?: boolean; timestamps?: { createdAt?: string; updatedAt?: string }; /** Empty or absent for applications submitted before JT-114. */ applicantName?: string }
export interface Resume { resumeId: string; fileName: string; validationStatus: string }
export interface Job { jobId: string; title: string; description: string; status: string; deadline: string }
// job_id is sent only when given, so calls without it are unchanged.
export const listApplications = (token: string, jobId?: string) =>
  api<{ applications?: Application[]; pagination?: Pagination }>(`/v1/applications?${pageQuery(token)}${jobId ? `&job_id=${encodeURIComponent(jobId)}` : ''}`);
export const listResumes = (token: string) => api<{ resumes?: Resume[]; pagination?: Pagination }>(`/v1/resumes?${pageQuery(token)}`);
export const getJob = (id: string) => api<{ job: Job }>(`/v1/jobs/${idPath(id)}`);
export const submitApplication = (jobId: string, resumeId: string) => api<{ application: Application }>('/v1/applications', { method: 'POST', body: { jobId, resumeId } });
export const withdrawApplication = (id: string) => api<{ application: Application }>(`/v1/applications/${idPath(id)}/withdraw`, { method: 'POST', body: {} });
// JT-112: one application by ID, for the applicant's application page.
export const getApplication = (id: string) => api<{ application: Application }>(`/v1/applications/${idPath(id)}`);
