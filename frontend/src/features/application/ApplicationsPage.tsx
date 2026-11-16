import { useState } from 'react';
import { Alert, Button, Card, CardContent, Dialog, DialogActions, DialogTitle, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { listApplications, withdrawApplication, type Application } from './api';
import { errorMessage, Pager, ResourceState, useResource } from './shared';

export function ApplicationsPage() {
  const employer = useAuth().session?.user.role === 'ROLE_EMPLOYER';
  const base = employer ? '/employer' : '/applicant';
  const [token, setToken] = useState('');
  const resource = useResource(() => listApplications(token), [token]);
  const [selected, setSelected] = useState<Application>();
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  async function withdraw() {
    if (!selected || busy) return;
    setBusy(true); setError(''); setNotice('');
    try { await withdrawApplication(selected.applicationId); setSelected(undefined); setNotice('Application withdrawn.'); resource.refresh(); }
    catch (reason) { setError(errorMessage(reason)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">{employer ? 'Applications for your jobs' : 'My applications'}</Typography>
    {!employer && <Button component={Link} to="/applicant/apply">Apply for a job</Button>}
    <Button disabled={busy || resource.loading} onClick={resource.refresh}>Refresh applications</Button>
    <ResourceState {...resource} />
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    {resource.data && !(resource.data.applications?.length) && <Typography>No applications on this page.</Typography>}
    {resource.data?.applications?.map(a => <Card key={a.applicationId}><CardContent><Stack spacing={1}>
      <Typography>Application: {a.applicationId}</Typography><Typography>Job: {a.jobId}</Typography>
      <Typography>{a.withdrawn ? 'Withdrawn' : 'Submitted (submission record)'}</Typography>
      <Button component={Link} to={`${base}/messages/${a.applicationId}`}>Messages</Button>
      <Button component={Link} to={`${base}/applications/${a.applicationId}/status`}>Status</Button>
      <Button component={Link} to={`${base}/applications/${a.applicationId}/interviews`}>Interviews</Button>
      {employer && <Button component={Link} to={`${base}/applications/${a.applicationId}/review`}>Review</Button>}
      {employer ? <Button component={Link} to={`${base}/offers/new/${a.applicationId}`}>Make offer</Button>
        : <Button disabled={busy || a.withdrawn} onClick={() => { setSelected(a); setError(''); }}>Withdraw</Button>}
      <Button component={Link} to={`${base}/report-job/${a.jobId}`}>Report job</Button>
    </Stack></CardContent></Card>)}
    <Pager next={resource.data?.pagination?.nextPageToken} token={token} busy={resource.loading || busy} onPage={setToken} />
    <Dialog open={!!selected} onClose={() => { if (!busy) setSelected(undefined); }}><DialogTitle>Withdraw this application?</DialogTitle><DialogActions>
      <Button disabled={busy} onClick={() => setSelected(undefined)}>Cancel</Button><Button disabled={busy} onClick={withdraw}>Confirm withdrawal</Button>
    </DialogActions></Dialog>
  </Stack>;
}
