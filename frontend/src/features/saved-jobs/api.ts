import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';
export interface SavedJob { jobId: string; applicantId: string; savedAt: string }
export const listSavedJobs = (token: string) => api<{ savedJobs?: SavedJob[]; pagination?: Pagination }>(`/v1/saved-jobs?${pageQuery(token)}`);
export const saveJob = (jobId: string) => api<{ savedJob: SavedJob }>('/v1/saved-jobs', { method: 'POST', body: { jobId } });
export const unsaveJob = (jobId: string) => api<Record<string, never>>(`/v1/saved-jobs/${idPath(jobId)}`, { method: 'DELETE' });
export const savedMessages = { 6: 'This job is already saved. Open Saved jobs to manage it.', 9: 'This job is closed and cannot be saved. Refresh the job.', 5: 'This bookmark is no longer available. Refresh Saved jobs.', 7: 'Only applicants can manage their own saved jobs.' };
