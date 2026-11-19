import { EmptyState } from '../../components/EmptyState';
import { useState } from 'react';
import { Alert, Button, Card, CardContent, Chip, Dialog, DialogActions, DialogTitle, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { listApplications, withdrawApplication, type Application } from './api';
import { errorMessage, Pager, ResourceState, useResource } from './shared';
import { PageHeader } from '../../components/PageHeader';

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
    <PageHeader title={employer ? 'Applications for your jobs' : 'My applications'} actions={<>
      <Button disabled={busy || resource.loading} onClick={resource.refresh}>Refresh applications</Button>
      {!employer && <Button variant="contained" component={Link} to="/applicant/apply">Apply for a job</Button>}
    </>} />
    <ResourceState {...resource} />
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    {resource.data && !(resource.data.applications?.length) && <EmptyState>No applications on this page.</EmptyState>}
    {resource.data?.applications?.map(a => <Card key={a.applicationId}><CardContent><Stack spacing={1}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1} sx={{ flexWrap: 'wrap' }}>
        <Typography variant="h6" component="h2">Application</Typography><Chip label={a.withdrawn ? 'Withdrawn' : 'Submitted (submission record)'} variant="outlined" color={a.withdrawn ? 'default' : 'primary'} />
      </Stack>
      <Typography className="record-id">Application: {a.applicationId}</Typography><Typography className="record-id">Job: {a.jobId}</Typography>
      <Stack className="record-actions">
      <Button component={Link} to={`${base}/messages/${a.applicationId}`}>Messages</Button>
      <Button component={Link} to={`${base}/applications/${a.applicationId}/status`}>Status</Button>
      <Button component={Link} to={`${base}/applications/${a.applicationId}/interviews`}>Interviews</Button>
      <Button component={Link} to={`${base}/offers?applicationId=${encodeURIComponent(a.applicationId)}`}>View Offer</Button>
      {employer && <Button variant="outlined" component={Link} to={`${base}/applications/${a.applicationId}/review`}>Review</Button>}
      {employer ? <Button variant="contained" component={Link} to={`${base}/offers/new/${a.applicationId}`}>Make offer</Button>
        : <Button color="error" disabled={busy || a.withdrawn} onClick={() => { setSelected(a); setError(''); }}>Withdraw</Button>}
      <Button component={Link} to={`${base}/report-job/${a.jobId}`}>Report job</Button>
      </Stack>
    </Stack></CardContent></Card>)}
    <Pager next={resource.data?.pagination?.nextPageToken} token={token} busy={resource.loading || busy} onPage={setToken} />
    <Dialog open={!!selected} onClose={() => { if (!busy) setSelected(undefined); }}><DialogTitle>Withdraw this application?</DialogTitle><DialogActions>
      <Button disabled={busy} onClick={() => setSelected(undefined)}>Cancel</Button><Button variant="contained" color="error" disabled={busy} onClick={withdraw}>Confirm withdrawal</Button>
    </DialogActions></Dialog>
  </Stack>;
}
