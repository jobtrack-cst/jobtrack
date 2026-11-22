import { useEffect, useState } from 'react';
import { getJob } from '../features/application/api';
import { isNotFound } from '../features/application-status/shared';
import type { Job } from '../features/job-posting/api';

// One GetJob per job for the whole app, so lists can show titles without
// repeated requests. Not found is remembered as null; other failures are retried.
const cache = new Map<string, Promise<Job | null>>();

export function loadJob(jobId: string): Promise<Job | null> {
  const key = jobId.toLowerCase();
  const cached = cache.get(key);
  if (cached) return cached;
  // GetJob returns the full job-posting Job; the application feature types only part of it.
  const request = getJob(key).then(r => (r.job as Job | undefined) ?? null, (error: unknown) => {
    cache.delete(key);
    if (isNotFound(error)) { cache.set(key, Promise.resolve(null)); return null; }
    throw error;
  });
  cache.set(key, request);
  return request;
}

// Lists that already hold full jobs (for example ListMyJobs) seed the cache.
export function rememberJob(job: Job) {
  if (job.jobId) cache.set(job.jobId.toLowerCase(), Promise.resolve(job));
}

export type JobState = { job?: Job; loading: boolean; missing: boolean; failed: boolean };

export function useJob(jobId?: string): JobState {
  const [state, setState] = useState<JobState & { id?: string }>({ loading: !!jobId, missing: false, failed: false });
  useEffect(() => {
    if (!jobId) return;
    let current = true;
    setState({ id: jobId, loading: true, missing: false, failed: false });
    loadJob(jobId).then(
      job => { if (current) setState({ id: jobId, job: job ?? undefined, loading: false, missing: job === null, failed: false }); },
      () => { if (current) setState({ id: jobId, loading: false, missing: false, failed: true }); });
    return () => { current = false; };
  }, [jobId]);
  if (!jobId) return { loading: false, missing: true, failed: false };
  return state.id === jobId ? state : { loading: true, missing: false, failed: false };
}

// The visible name of a job: its title, or a plain placeholder. Never the ID.
export function jobName({ job, loading, missing }: JobState) {
  if (job?.title) return job.title;
  if (loading) return 'Loading job…';
  return missing ? 'Job no longer available' : 'Job details unavailable';
}
