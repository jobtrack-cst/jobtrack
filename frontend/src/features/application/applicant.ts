import { isFinalStatus } from '../../components/StatusRail';
import { getStatus } from '../application-status/api';
import { isNotFound } from '../application-status/shared';
import { listApplications, type Application } from './api';
import type { Pagination } from './shared';

// Lists are walked to the end with a page limit and a repeated-token guard.
const maxPages = 50;
export async function allPages<T>(load: (token: string) => Promise<{ items?: T[]; pagination?: Pagination }>) {
  const items: T[] = [];
  const seen = new Set<string>();
  let token = '';
  for (let page = 0; page < maxPages; page++) {
    const result = await load(token);
    items.push(...(result.items ?? []));
    token = result.pagination?.nextPageToken ?? '';
    if (!token || seen.has(token)) break;
    seen.add(token);
  }
  return items;
}

// A missing status record is expected right after submitting; the event arrives shortly.
// A withdrawn application is final even before its status event is processed.
export async function statusOf(app: Application): Promise<string> {
  if (app.withdrawn) return 'APPLICATION_STATUS_WITHDRAWN';
  try { return (await getStatus(app.applicationId)).statusRecord?.status ?? 'APPLICATION_STATUS_SUBMITTED'; }
  catch (error) { if (isNotFound(error)) return 'APPLICATION_STATUS_SUBMITTED'; throw error; }
}

export interface Tracked { app: Application; status: string; active: boolean }
const track = async (app: Application): Promise<Tracked> => { const status = await statusOf(app); return { app, status, active: !isFinalStatus(status) }; };

const newestFirst = (a: Application, b: Application) => (Date.parse(b.timestamps?.createdAt ?? '') || 0) - (Date.parse(a.timestamps?.createdAt ?? '') || 0);

// Every application of the signed-in applicant, one per job (the newest), with its status.
export async function loadMyApplications(): Promise<Tracked[]> {
  const all = await allPages(token => listApplications(token).then(r => ({ items: r.applications, pagination: r.pagination })));
  const seen = new Set<string>();
  const newest = [...all].sort(newestFirst).filter(a => { const key = a.jobId.toLowerCase(); if (seen.has(key)) return false; seen.add(key); return true; });
  return Promise.all(newest.map(track));
}

// The applicant's application for one job, or null. The service allows one per job.
export async function findApplicationForJob(jobId: string): Promise<Tracked | null> {
  const { applications = [] } = await listApplications('', jobId);
  const app = [...applications].sort(newestFirst)[0];
  return app ? track(app) : null;
}
