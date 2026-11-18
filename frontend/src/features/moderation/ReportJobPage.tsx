import { useState } from 'react';
import { Alert, Button, Stack, TextField, Typography } from '@mui/material';
import { useParams } from 'react-router';
import { validId } from '../application/shared';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { reportJob } from './api';
import { readyToSend, reasonProblems } from './rules';

const messages = {
  [grpcCode.invalidArgument]: 'The report was not accepted. Use a valid job ID and a reason of 1 to 1000 characters.',
  [grpcCode.alreadyExists]: 'You already have an open report for this job. An admin will review it.',
  [grpcCode.failedPrecondition]: 'This job has already been removed, so it cannot be reported.',
};

export function ReportJobPage() {
  const { jobId = '' } = useParams();
  // The key resets the form when the address names another job.
  return <ReportForm key={jobId} initialJobId={validId(jobId) ? jobId.toLowerCase() : ''} badAddress={jobId !== '' && !validId(jobId)} />;
}

function ReportForm({ initialJobId, badAddress }: { initialJobId: string; badAddress: boolean }) {
  const [jobId, setJobId] = useState(initialJobId);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const problems = reasonProblems(jobId, reason, 1000);
  const ready = readyToSend(jobId, reason, problems);
  async function submit() {
    if (busy || !ready) return;
    setBusy(true); setError(''); setNotice('');
    try {
      await reportJob(jobId.trim().toLowerCase(), reason.trim());
      setReason('');
      setNotice('Report sent. An admin will review it.');
    } catch (failure) { setError(gatewayMessage(failure, messages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Report a job</Typography>
    <Typography>Report a job posting that looks false, misleading or inappropriate. An admin reviews every report and can remove the job.</Typography>
    {badAddress && <Alert severity="warning">The job ID in the address is not valid. Enter the job ID below.</Alert>}
    <Stack component="form" spacing={2} noValidate onSubmit={e => { e.preventDefault(); void submit(); }}>
      <TextField label="Job ID" required value={jobId} disabled={busy} onChange={e => { setJobId(e.target.value); setNotice(''); }} />
      <TextField label="Reason" required multiline minRows={3} value={reason} disabled={busy} helperText="1 to 1000 characters"
        onChange={e => { setReason(e.target.value); setNotice(''); }} />
      {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
      {error && <Alert severity="error">{error}</Alert>}
      {notice && <Alert severity="success">{notice}</Alert>}
      <Button type="submit" variant="contained" disabled={busy || !ready}>Send report</Button>
    </Stack>
  </Stack>;
}
