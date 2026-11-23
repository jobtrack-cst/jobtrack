import { useEffect, useState } from 'react';
import { listSavedJobs } from './api';

// Which jobs the signed-in applicant has saved, so Find jobs and job details can
// show Save or Unsave. One walk through every ListSavedJobs page per user, then
// kept up to date by Save and Unsave. A failed load is retried on the next use.
const maxPages = 50;
let cached: { userId: string; request: Promise<Set<string>> } | null = null;
const listeners = new Set<() => void>();
const notify = () => listeners.forEach(listener => listener());

async function loadAll() {
  const ids = new Set<string>();
  const seen = new Set<string>();
  let token = '';
  for (let page = 0; page < maxPages; page++) {
    const { savedJobs = [], pagination } = await listSavedJobs(token);
    savedJobs.forEach(s => ids.add(s.jobId.toLowerCase()));
    token = pagination?.nextPageToken ?? '';
    if (!token || seen.has(token)) break;
    seen.add(token);
  }
  return ids;
}

export function loadSavedIds(userId: string): Promise<Set<string>> {
  if (cached?.userId === userId) return cached.request;
  const request = loadAll();
  cached = { userId, request };
  request.catch(() => { if (cached?.request === request) cached = null; });
  return request;
}

export function markSaved(userId: string, jobId: string, saved: boolean) {
  // Nothing cached (for example after a failed load): read the list again, which includes the change.
  if (cached?.userId !== userId) { cached = null; notify(); return; }
  const key = jobId.toLowerCase();
  cached = { userId, request: cached.request.then(ids => { const next = new Set(ids); if (saved) next.add(key); else next.delete(key); return next; }) };
  notify();
}

// The Saved page reads the server directly; afterwards the next lookup starts fresh.
export function forgetSavedIds() {
  cached = null;
}

// undefined while loading or when the list could not be read.
export function useSavedIds(userId?: string) {
  const [state, setState] = useState<{ userId?: string; ids?: Set<string> }>({});
  const [version, setVersion] = useState(0);
  useEffect(() => { const listener = () => setVersion(v => v + 1); listeners.add(listener); return () => { listeners.delete(listener); }; }, []);
  useEffect(() => {
    if (!userId) return;
    let current = true;
    loadSavedIds(userId).then(ids => { if (current) setState({ userId, ids }); }, () => { if (current) setState({ userId }); });
    return () => { current = false; };
  }, [userId, version]);
  return state.userId === userId ? state.ids : undefined;
}
