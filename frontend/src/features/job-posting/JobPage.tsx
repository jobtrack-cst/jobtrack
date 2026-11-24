import { useState } from 'react';
import { Alert, Button, Stack, Typography } from '@mui/material';
import { Link, useLocation, useNavigate, useParams } from 'react-router';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { FormPanel } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { rememberJob } from '../../components/useJob';
import { useAuth } from '../auth/AuthProvider';
import { getJob } from '../application/api';
import { validId } from '../application/shared';
import { gatewayMessage } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { closeJob, createJob, deleteJob, publishJob, updateJob, type Job, type JobInput } from './api';
import { JobDetails } from './JobDetails';
import { JobForm } from './JobForm';
import { deleteMessages, editable, jobMessages, publishMessages, useJobReferences } from './shared';

type Confirm = 'publish' | 'close' | 'delete' | '';
const confirmText = {
  publish: { title: 'Publish this job?', label: 'Confirm publish', body: 'Applicants can find and apply for it until its deadline.' },
  close: { title: 'Close this job?', label: 'Confirm close', body: 'Applicants can no longer apply. Closing cannot be undone.' },
  delete: { title: 'Delete this draft?', label: 'Confirm delete', body: 'The draft is deleted for good. This cannot be undone.' },
};

const back = { to: '/employer/jobs', label: 'Jobs' };

// One job for its owner. /employer/jobs/new opens the editor; an existing job opens
// read-only with Edit and Publish or Close (JT-109 pattern), and a draft also with
// Delete draft (JT-116). Closed and removed jobs are read-only only.
export function JobPage({ create = false }: { create?: boolean }) {
  const { jobId = '' } = useParams();
  if (create) return <NewJob />;
  // An address that is not a job ID never reaches the gateway.
  if (!validId(jobId)) return <Stack spacing={4}>
    <PageHeader back={back} title="Job not found" />
    <EmptyState title="This job does not exist" description="The link may be incomplete. Open the job from Jobs."
      action={<Button component={Link} to="/employer/jobs" variant="outlined">Jobs</Button>} />
  </Stack>;
  return <JobLoader key={jobId.toLowerCase()} jobId={jobId.toLowerCase()} />;
}

function NewJob() {
  const navigate = useNavigate();
  const references = useJobReferences();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function save(input: JobInput) {
    if (busy) return;
    setBusy(true); setError('');
    try {
      const { job } = await createJob(input);
      rememberJob(job);
      navigate(`/employer/jobs/${encodeURIComponent(job.jobId)}`, { replace: true, state: { notice: 'Draft created. Publish it when it is ready.' } });
    } catch (reason) { setError(gatewayMessage(reason, jobMessages)); setBusy(false); }
  }
  return <Stack spacing={4}>
    <PageHeader back={back} title="Post a job" description="A new job is saved as a draft. Publish it when it is ready." />
    {references.loading && <LoadingRows rows={3} label="Loading job options" />}
    {references.error && <ErrorState message={references.error} onRetry={references.refresh} />}
    {references.data && <FormPanel>
      <Stack spacing={2.5}>
        <JobForm references={references.data} busy={busy} onSave={save} />
        {error && <Alert severity="error">{error}</Alert>}
      </Stack>
    </FormPanel>}
  </Stack>;
}

function JobLoader({ jobId }: { jobId: string }) {
  const { session } = useAuth();
  const resource = useLoad(() => getJob(jobId).then(r => r.job as Job), [jobId], jobMessages);
  if (resource.loading) return <Stack spacing={4}><PageHeader back={back} title="Loading job…" /><LoadingRows rows={2} height={160} label="Loading job" /></Stack>;
  if (!resource.data) return <Stack spacing={4}><PageHeader back={back} title="Job" />
    <ErrorState message={resource.error || 'This job was not found. Open it again from Jobs.'} onRetry={resource.refresh} /></Stack>;
  if (resource.data.employerId !== session?.user.userId) return <Stack spacing={4}><PageHeader back={back} title="Job" />
    <Alert severity="error">Only the employer who owns this job can manage it.</Alert></Stack>;
  return <JobView initial={resource.data} />;
}

function JobView({ initial }: { initial: Job }) {
  const location = useLocation();
  const navigate = useNavigate();
  const references = useJobReferences();
  const [job, setJob] = useState(initial);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState((location.state as { notice?: string } | null)?.notice ?? '');
  const [confirm, setConfirm] = useState<Confirm>('');
  const [transitionError, setTransitionError] = useState('');
  const canEdit = editable(job.status);
  function show(next: Job) { setJob(next); rememberJob(next); }
  async function save(input: JobInput) {
    if (busy) return;
    setBusy(true); setError(''); setNotice('');
    try { show((await updateJob(job.jobId, input)).job); setEditing(false); setNotice('Job updated.'); }
    catch (reason) { setError(gatewayMessage(reason, jobMessages)); }
    finally { setBusy(false); }
  }
  async function transition() {
    if (busy || !confirm) return;
    setBusy(true); setTransitionError(''); setNotice('');
    if (confirm === 'delete') {
      try {
        await deleteJob(job.jobId);
        navigate('/employer/jobs?tab=drafts', { state: { notice: 'Draft deleted.' } });
      } catch (reason) { setTransitionError(gatewayMessage(reason, deleteMessages)); setBusy(false); }
      return;
    }
    try {
      show((await (confirm === 'publish' ? publishJob : closeJob)(job.jobId)).job);
      setNotice(confirm === 'publish' ? 'Job published. Search results may take a moment to update.' : 'Job closed. Search and saved jobs may take a moment to update.');
      setConfirm('');
    } catch (reason) { setTransitionError(gatewayMessage(reason, confirm === 'publish' ? publishMessages : jobMessages)); }
    finally { setBusy(false); }
  }
  function openConfirm(action: Exclude<Confirm, ''>) { setNotice(''); setTransitionError(''); setConfirm(action); }
  const actions = canEdit && !editing && <>
    <Button variant="outlined" disabled={busy} onClick={() => { setNotice(''); setError(''); setEditing(true); }}>Edit job</Button>
    {job.status === 'JOB_STATUS_DRAFT' && <Button variant="outlined" color="error" disabled={busy} onClick={() => openConfirm('delete')}>Delete draft</Button>}
    {job.status === 'JOB_STATUS_DRAFT' && <Button variant="contained" disabled={busy} onClick={() => openConfirm('publish')}>Publish job</Button>}
    {job.status === 'JOB_STATUS_PUBLISHED' && <Button variant="outlined" color="error" disabled={busy} onClick={() => openConfirm('close')}>Close job</Button>}
  </>;
  return <Stack spacing={4}>
    <PageHeader back={back} title={job.title || 'Untitled job'} action={actions || undefined} />
    {notice && <Alert severity="success" role="status">{notice}</Alert>}
    {job.status === 'JOB_STATUS_CLOSED' && <Alert severity="info">This job is closed. Closed jobs cannot be edited or published again.</Alert>}
    {job.status === 'JOB_STATUS_REMOVED' && <Alert severity="info">An admin removed this job. Removed jobs cannot be edited.</Alert>}
    {references.loading && <LoadingRows rows={3} label="Loading job options" />}
    {references.error && <ErrorState message={references.error} onRetry={references.refresh} />}
    {references.data && <FormPanel>
      {editing
        ? <Stack spacing={2.5}>
          <Typography variant="h2" component="h2">Edit job</Typography>
          <JobForm key={`${job.jobId}:${job.timestamps?.updatedAt ?? ''}`} job={job} references={references.data} busy={busy} onSave={save}
            onCancel={() => { setError(''); setEditing(false); }} />
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
        : <JobDetails job={job} categories={references.data.categories} tags={references.data.tags} />}
    </FormPanel>}
    <ConfirmDialog open={!!confirm} busy={busy} error={transitionError} destructive={confirm === 'close' || confirm === 'delete'}
      title={confirmText[confirm || 'publish'].title}
      confirmLabel={confirmText[confirm || 'publish'].label}
      onConfirm={() => void transition()} onCancel={() => setConfirm('')}>
      <Typography>{confirmText[confirm || 'publish'].body}</Typography>
    </ConfirmDialog>
  </Stack>;
}
