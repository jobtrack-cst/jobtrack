import { listJobs, type Job, type JobStatus } from '../job-posting/api';
import type { Application } from './api';
import { allPages } from './applicant';

// The applicant's name stored at submission (JT-114 Part A). Applications from before
// JT-114 have none, and the UI then says "Applicant"; it never invents a name.
export const applicantLabel = (app: Pick<Application, 'applicantName'>) => app.applicantName?.trim() || 'Applicant';

// Jobs that can have applicants (drafts cannot), for the Applicants job filter:
// active jobs first, then closed, then removed, each newest first as ListMyJobs returns them.
const filterStatuses: [JobStatus, string][] = [['JOB_STATUS_PUBLISHED', ''], ['JOB_STATUS_CLOSED', ' (Closed)'], ['JOB_STATUS_REMOVED', ' (Removed)']];

export interface JobOption { job: Job; label: string }

export async function loadFilterJobs(): Promise<JobOption[]> {
  const lists = await Promise.all(filterStatuses.map(([status]) =>
    allPages(token => listJobs(token, status).then(r => ({ items: r.jobs, pagination: r.pagination })))));
  return lists.flatMap((jobs, i) => jobs.map(job => ({ job, label: `${job.title || 'Untitled job'}${filterStatuses[i][1]}` })));
}
