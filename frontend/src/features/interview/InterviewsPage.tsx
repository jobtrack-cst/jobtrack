import { useState } from 'react';
import { Alert, Button, Card, CardContent, Stack, TextField, Typography } from '@mui/material';
import { Link, useParams } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { Pager, ResourceState, validId } from '../application/shared';
import { formatTime, gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listInterviews, scheduleInterview } from './api';
import { checkSchedule, maxDetails } from './schedule';

const listMessages = {
  [grpcCode.notFound]: 'This application is not available yet. It appears shortly after it is submitted; refresh to try again.',
  [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see its interviews.',
};
const scheduleMessages = {
  [grpcCode.invalidArgument]: 'The interview was not scheduled. Choose a time in the future within 365 days, with details of up to 1000 characters.',
  [grpcCode.alreadyExists]: 'An interview is already scheduled at this time for this application.',
  [grpcCode.permissionDenied]: 'Only the employer who posted this job can schedule interviews.',
  [grpcCode.notFound]: listMessages[grpcCode.notFound],
};

export function InterviewsPage() {
  const { applicationId = '' } = useParams();
  if (!validId(applicationId)) return <Alert severity="error">The application ID in the address is not valid.</Alert>;
  return <InterviewsView applicationId={applicationId.toLowerCase()} />;
}

function InterviewsView({ applicationId }: { applicationId: string }) {
  const employer = useAuth().session?.user.role === 'ROLE_EMPLOYER';
  const base = employer ? '/employer' : '/applicant';
  const [token, setToken] = useState('');
  const list = useLoad(() => listInterviews(applicationId, token), [applicationId, token], listMessages);
  const [when, setWhen] = useState('');
  const [details, setDetails] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const checked = checkSchedule(when, details);
  async function schedule() {
    if (busy) return;
    const now = checkSchedule(when, details);
    if (!now.ok) { setError(now.message); return; }
    setBusy(true); setError(''); setNotice('');
    try {
      const { interview } = await scheduleInterview(applicationId, now.scheduledAt, details.trim());
      setNotice(`Interview scheduled for ${formatTime(interview.scheduledAt)}.`); setWhen(''); setDetails(''); setToken(''); list.refresh();
    } catch (reason) { setError(gatewayMessage(reason, scheduleMessages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Interviews</Typography>
    <Typography variant="body2">Application: {applicationId}</Typography>
    <Button component={Link} to={`${base}/applications/${applicationId}/status`}>View status</Button>
    <Button disabled={busy || list.loading} onClick={list.refresh}>Refresh interviews</Button>
    <ResourceState loading={list.loading} error={list.error} />
    {list.data && !list.data.interviews?.length && <Typography>No interviews scheduled on this page.</Typography>}
    {list.data?.interviews?.map(i => <Card key={i.interviewId}><CardContent><Stack spacing={1}>
      <Typography variant="h6" component="h2">{formatTime(i.scheduledAt)}</Typography>
      <Typography sx={{ whiteSpace: 'pre-wrap' }}>{i.details || 'No details given.'}</Typography>
    </Stack></CardContent></Card>)}
    <Pager next={list.data?.pagination?.nextPageToken} token={token} busy={busy || list.loading} onPage={setToken} />
    {employer && <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); void schedule(); }}>
      <Typography variant="h6" component="h2">Schedule an interview</Typography>
      <TextField label="Date and time" type="datetime-local" required value={when} disabled={busy} onChange={e => setWhen(e.target.value)}
        helperText="Your local time. Must be in the future and within 365 days." slotProps={{ inputLabel: { shrink: true } }} />
      <TextField label="Details" multiline minRows={3} value={details} disabled={busy} onChange={e => setDetails(e.target.value)}
        helperText={`For example the place or meeting link. ${length(details)} of ${maxDetails} characters.`} />
      {when && !checked.ok && <Alert severity="warning">{checked.message}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
      <Button type="submit" variant="contained" disabled={busy || !checked.ok}>Schedule interview</Button>
    </Stack>}
    {!employer && <Typography variant="body2">The employer schedules interviews; they appear here.</Typography>}
  </Stack>;
}
