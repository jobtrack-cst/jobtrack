import { useState } from 'react';
import { Alert, Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, gatewayMessage } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listSavedJobs, savedMessages, unsaveJob } from './api';

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
    <Typography variant="h4" component="h1">Saved jobs</Typography><Typography>Closed jobs are removed after their closure event is processed. Open View job for the latest details.</Typography>
    <ResourceState {...resource} />{error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    <Button disabled={busy || resource.loading} onClick={() => { setError(''); resource.refresh(); }}>Refresh saved jobs</Button>
    {resource.data && <Typography>Saved jobs: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.savedJobs?.length && <Typography>No saved jobs on this page.</Typography>}
    {resource.data?.savedJobs?.map(job => <Card key={job.jobId}><CardContent><Stack spacing={1}>
      <Typography>Job ID: {job.jobId}</Typography><Typography>Saved: {formatTime(job.savedAt)}</Typography>
      <Button component={Link} to={`/applicant/jobs/${job.jobId}`}>View job</Button>
      <Button disabled={busy} onClick={() => void remove(job.jobId)}>Unsave job</Button>
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={busy || resource.loading} onPage={setToken} />
  </Stack>;
}
