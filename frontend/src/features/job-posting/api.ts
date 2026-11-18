import { api } from '../../api/client';
import type { Job as ApplicationJob } from '../application/api';
import { idPath, pageQuery, type Pagination } from '../application/shared';
export interface Job extends ApplicationJob { employerId: string; categoryId?: string; tagIds?: string[]; location?: string }
export type JobInput = Pick<Job, 'title' | 'description' | 'categoryId' | 'tagIds' | 'location' | 'deadline'>;
export const listJobs = (token: string) => api<{ jobs?: Job[]; pagination?: Pagination }>(`/v1/employer/jobs?${pageQuery(token)}`);
export const createJob = (body: JobInput) => api<{ job: Job }>('/v1/jobs', { method: 'POST', body });
export const updateJob = (id: string, body: JobInput) => api<{ job: Job }>(`/v1/jobs/${idPath(id)}`, { method: 'PUT', body });
export const publishJob = (id: string) => api<{ job: Job }>(`/v1/jobs/${idPath(id)}/publish`, { method: 'POST', body: {} });
export const closeJob = (id: string) => api<{ job: Job }>(`/v1/jobs/${idPath(id)}/close`, { method: 'POST', body: {} });
