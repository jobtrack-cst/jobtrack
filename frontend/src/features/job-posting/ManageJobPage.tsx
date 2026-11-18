import { useState } from 'react';
import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, Typography } from '@mui/material';
import { Link, useNavigate, useParams } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { getJob as loadJob } from '../application/api';
import { ResourceState, validId } from '../application/shared';
import { formatTime, gatewayMessage } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { closeJob, createJob, publishJob, updateJob, type Job, type JobInput } from './api';
import { JobForm } from './JobForm';
import { editable, jobMessages, jobStatus, publishMessages } from './shared';

export function ManageJobPage({ create = false }: { create?: boolean }) {
  const { jobId = '' } = useParams();
  const navigate = useNavigate();
  const { session } = useAuth();
  const resource = useLoad(async () => create || !validId(jobId) ? undefined : (await loadJob(jobId)).job as Job, [jobId, create], jobMessages);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [transitionError, setTransitionError] = useState('');
  const [confirm, setConfirm] = useState<'publish' | 'close' | ''>('');
  async function save(input: JobInput) {
    if (busy || (!create && resource.data?.employerId !== session?.user.userId)) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const result = create ? await createJob(input) : await updateJob(jobId, input);
      if (create) navigate(`/employer/jobs/${result.job.jobId}`, { replace: true });
      else { setNotice('Job updated.'); resource.refresh(); }
    } catch (reason) { setError(gatewayMessage(reason, jobMessages)); }
    finally { setBusy(false); }
  }
  async function transition() {
    if (busy || !confirm || resource.data?.employerId !== session?.user.userId) return;
    setBusy(true); setError(''); setTransitionError(''); setNotice('');
    try {
      if (confirm === 'publish') await publishJob(jobId); else await closeJob(jobId);
      setNotice(confirm === 'publish' ? 'Job published. Search results may take a moment to update.' : 'Job closed. Search and saved jobs may take a moment to update.');
      setConfirm(''); resource.refresh();
    } catch (reason) { setTransitionError(gatewayMessage(reason, confirm === 'publish' ? publishMessages : jobMessages)); }
    finally { setBusy(false); }
  }
  const ownJob = resource.data?.employerId === session?.user.userId;
  const job = ownJob ? resource.data : undefined;
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">{create ? 'Create job' : 'Manage job'}</Typography>
    <Button component={Link} to="/employer/jobs">My jobs</Button>
    {!create && !validId(jobId) && <Alert severity="error">The job ID in the address is not valid.</Alert>}
    <ResourceState {...resource} />
    {resource.data && !ownJob && <Alert severity="error">Only the employer who owns this job can manage it.</Alert>}
    {!create && <Button disabled={busy || resource.loading} onClick={() => { setError(''); resource.refresh(); }}>Refresh job</Button>}
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    {job && <><Typography>Status: {jobStatus(job.status)}</Typography>
      <Typography>Deadline: {formatTime(job.deadline)}</Typography>
      {job.status === 'JOB_STATUS_DRAFT' && <Button disabled={busy} onClick={() => { setTransitionError(''); setConfirm('publish'); }}>Publish job</Button>}
      {job.status === 'JOB_STATUS_PUBLISHED' && <Button disabled={busy} onClick={() => { setTransitionError(''); setConfirm('close'); }}>Close job</Button>}
      {!editable(job.status) && <><Typography variant="h6">{job.title}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{job.description}</Typography><Alert severity="info">Closed and removed jobs cannot be edited.</Alert></>}
    </>}
    {(create || (job && editable(job.status))) && <JobForm key={job ? `${job.jobId}:${job.title}:${job.deadline}` : 'new'} job={job} busy={busy} onSave={save} />}
    <Dialog open={!!confirm} onClose={() => { if (!busy) setConfirm(''); }}>
      <DialogTitle>{confirm === 'publish' ? 'Publish this job?' : 'Close this job? Closing cannot be undone.'}</DialogTitle>
      <DialogContent>{transitionError && <Alert severity="error">{transitionError}</Alert>}</DialogContent>
      <DialogActions><Button disabled={busy} onClick={() => setConfirm('')}>Cancel</Button><Button disabled={busy} onClick={() => void transition()}>Confirm {confirm}</Button></DialogActions>
    </Dialog>
  </Stack>;
}
