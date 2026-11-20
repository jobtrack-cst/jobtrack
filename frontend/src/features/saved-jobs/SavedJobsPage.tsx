import { EmptyState } from '../../components/EmptyState';
import { useState } from 'react';
import { Alert, Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, gatewayMessage } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listSavedJobs, savedMessages, unsaveJob } from './api';
import { PageHeader } from '../../components/PageHeader';

export function SavedJobsPage() {
  const [token, setToken] = useState('');
  const resource = useLoad(() => listSavedJobs(token), [token], savedMessages);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function remove(id: string) {
    if (busy) return;
    setBusy(true); setError(''); setNotice('');
    try { await unsaveJob(id); setNotice('Job unsaved.'); setToken(''); resource.refresh(); }
    catch (reason) { setError(gatewayMessage(reason, savedMessages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <PageHeader title="Saved jobs" description="Closed jobs are removed after their closure event is processed. Open View job for the latest details."
      actions={<Button disabled={busy || resource.loading} onClick={() => { setError(''); resource.refresh(); }}>Refresh saved jobs</Button>} />
    <ResourceState {...resource} />{error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    {resource.data && <Typography variant="subtitle2" color="text.secondary">Saved jobs: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.savedJobs?.length && <EmptyState>No saved jobs on this page.</EmptyState>}
    {resource.data?.savedJobs?.map(job => <Card key={job.jobId}><CardContent><Stack spacing={1}>
      <Typography variant="h6" component="h2">Saved opportunity</Typography>
      <Typography className="record-id">Job ID: {job.jobId}</Typography><Typography variant="body2" color="text.secondary">Saved: {formatTime(job.savedAt)}</Typography>
      <Stack className="record-actions">
        <Button variant="outlined" component={Link} to={`/applicant/jobs/${job.jobId}`}>View job</Button>
        <Button color="error" disabled={busy} onClick={() => void remove(job.jobId)}>Unsave job</Button>
      </Stack>
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={busy || resource.loading} onPage={setToken} />
  </Stack>;
}
