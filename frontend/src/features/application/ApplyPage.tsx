import { useState } from 'react';
import { Alert, Button, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { Link, useSearchParams } from 'react-router';
import { getJob, listResumes, submitApplication, type Application } from './api';
import { errorMessage, Pager, ResourceState, useResource, validId } from './shared';

export function ApplyPage() {
  const [params] = useSearchParams();
  const [jobId, setJobId] = useState(params.get('jobId') ?? '');
  const [loadedId, setLoadedId] = useState('');
  const [token, setToken] = useState('');
  const resumes = useResource(() => listResumes(token), [token]);
  const job = useResource(() => loadedId ? getJob(loadedId) : Promise.resolve(undefined), [loadedId]);
  const [resumeId, setResumeId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<Application>();
  const open = job.data?.job.status === 'JOB_STATUS_PUBLISHED' && Date.parse(job.data.job.deadline) > Date.now();
  const validated = resumes.data?.resumes?.filter(r => r.validationStatus === 'RESUME_VALIDATION_STATUS_VALIDATED') ?? [];
  async function apply() {
    if (busy || !open || !validated.some(r => r.resumeId === resumeId)) return;
    setBusy(true); setError('');
    try { setResult((await submitApplication(loadedId, resumeId)).application); }
    catch (reason) { setError(errorMessage(reason, 'apply')); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Apply for a job</Typography>
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); setLoadedId(jobId.trim()); job.refresh(); setResult(undefined); setError(''); }}>
      <TextField label="Job ID" value={jobId} disabled={busy} onChange={e => { setJobId(e.target.value); setLoadedId(''); setResult(undefined); }} required />
      <Button type="submit" disabled={busy || !validId(jobId.trim())}>Load job</Button>
    </Stack>
    <ResourceState {...job} />
    {job.data && <><Typography variant="h6">{job.data.job.title}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{job.data.job.description}</Typography>
      <Typography>Deadline: {new Date(job.data.job.deadline).toLocaleString()}</Typography>
      {!open && <Alert severity="warning">This job is closed, unavailable for applications, or past its deadline.</Alert>}</>}
    <ResourceState {...resumes} />
    <Button disabled={busy || resumes.loading} onClick={() => { setResumeId(''); resumes.refresh(); }}>Refresh resumes</Button>
    {resumes.data && validated.length === 0 && <Alert severity="info">No validated resume on this page. Upload a PDF from your resume workspace and wait for validation, then refresh or check the next page.</Alert>}
    <TextField select label="Validated resume" value={resumeId} disabled={busy || resumes.loading} onChange={e => setResumeId(e.target.value)}>
      {validated.map(r => <MenuItem key={r.resumeId} value={r.resumeId}>{r.fileName}</MenuItem>)}
    </TextField>
    <Pager token={token} next={resumes.data?.pagination?.nextPageToken} busy={busy || resumes.loading} onPage={value => { setToken(value); setResumeId(''); }} />
    {error && <Alert severity="error">{error}</Alert>}
    {result && <Alert severity="success">Application submitted. ID: {result.applicationId}. <Link to="/applicant/applications">My applications</Link></Alert>}
    <Button variant="contained" disabled={busy || !!result || !open || !validated.some(r => r.resumeId === resumeId)} onClick={apply}>Submit application</Button>
  </Stack>;
}
