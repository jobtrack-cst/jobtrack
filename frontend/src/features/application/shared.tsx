import { useEffect, useState } from 'react';
import { Alert, Button, CircularProgress, Stack } from '@mui/material';
import { ApiError } from '../../api/client';

export const idPath = (id: string) => encodeURIComponent(id);
export const validId = (id: string) => /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id);
export interface Pagination { nextPageToken?: string; totalCount?: number }
export const pageQuery = (token = '') => `pagination.page_size=20&pagination.page_token=${encodeURIComponent(token)}`;
export function errorMessage(error: unknown, context = '') {
  if (error instanceof ApiError) {
    if (error.status === 404) return 'This record was not found or is not available yet. Check the ID or refresh shortly.';
    if (error.status === 409) return context === 'apply' ? 'You have already applied for this job.' : context === 'alert' ? 'This subscription already exists.' : 'An offer already exists for this application. Only one lifetime offer is allowed.';
    if (error.status === 400 && error.code === 9) return context === 'apply'
      ? 'Cannot apply: the job may be closed or past its deadline, or your resume is not validated. Refresh the job and resumes before retrying.'
      : context === 'alert' ? 'You can have at most 10 subscriptions. Delete one before adding another.'
      : 'This action is no longer allowed. Refresh to see the latest state.';
  }
  return error instanceof Error ? error.message : 'The request failed. Please try again.';
}
export function useResource<T>(load: () => Promise<T>, dependencies: unknown[]) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true); setError(''); setData(undefined);
    load().then(value => { if (active) setData(value); }, reason => { if (active) setError(errorMessage(reason)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
    // Dependencies are supplied by each resource's caller.
  }, [...dependencies, revision]);
  return { data, error, loading, refresh: () => setRevision(n => n + 1) };
}
export function ResourceState({ loading, error }: { loading: boolean; error: string }) {
  return <>{loading && <CircularProgress aria-label="Loading" size={24} />}{error && <Alert severity="error">{error}</Alert>}</>;
}
export function Pager({ next, token, busy, onPage }: { next?: string; token: string; busy: boolean; onPage: (token: string) => void }) {
  return <Stack direction="row" spacing={2}><Button disabled={busy || !token} onClick={() => onPage('')}>First page</Button><Button disabled={busy || !next} onClick={() => onPage(next!)}>Next page</Button></Stack>;
}
