import { useState } from 'react';
import { Alert, Button, Stack, TextField, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listMyVerificationRequests, submitVerificationRequest } from './api';
import { RequestCard } from './RequestCard';

const listMessages = { [grpcCode.permissionDenied]: 'Only employers can view verification requests.' };
const submitMessages = {
  [grpcCode.invalidArgument]: 'The request was not accepted. The registration number needs 1 to 50 characters and notes at most 1000.',
  [grpcCode.alreadyExists]: 'You already have a pending request. Wait for the admin decision before sending another.',
  [grpcCode.failedPrecondition]: 'Your company is already verified, so no new request is needed.',
  [grpcCode.permissionDenied]: 'Only employers can request verification.',
};

export function VerificationPage() {
  const [token, setToken] = useState('');
  const list = useLoad(() => listMyVerificationRequests(token), [token], listMessages);
  const [registration, setRegistration] = useState('');
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // Requests are newest first, so the first one on the first page is the latest.
  const latest = token === '' ? list.data?.requests?.[0] : undefined;
  const pending = latest?.status === 'REQUEST_STATUS_PENDING';
  const approved = latest?.status === 'REQUEST_STATUS_APPROVED';
  const canRequest = token === '' && !!list.data && !pending && !approved;
  const problems: string[] = [];
  if (length(registration) > 50) problems.push('The registration number can have at most 50 characters.');
  if (length(notes) > 1000) problems.push('Notes can have at most 1000 characters.');
  const ready = length(registration) > 0 && problems.length === 0;
  async function submit() {
    if (busy || !ready) return;
    setBusy(true); setError(''); setNotice('');
    try {
      await submitVerificationRequest(registration.trim(), notes.trim());
      setRegistration(''); setNotes('');
      setNotice('Request sent. An admin will approve or reject it, and your company profile shows the result.');
      list.refresh();
    } catch (reason) { setError(gatewayMessage(reason, submitMessages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Company verification</Typography>
    <Typography>An admin checks your company registration before you can publish jobs. The result appears on your company profile, so create the profile first.</Typography>
    <Button component={Link} to="/employer/company">Company profile</Button>
    {error && <Alert severity="error">{error}</Alert>}
    {notice && <Alert severity="success">{notice}</Alert>}
    {pending && <Alert severity="info">Your latest request is pending. An admin will approve or reject it.</Alert>}
    {approved && <Alert severity="success">Your company is verified. No new request is needed.</Alert>}
    {canRequest && <Stack component="form" spacing={2} noValidate onSubmit={e => { e.preventDefault(); void submit(); }}>
      <Typography variant="h6" component="h2">{latest?.status === 'REQUEST_STATUS_REJECTED' ? 'Send a new request' : 'Request verification'}</Typography>
      <TextField label="Registration number" required value={registration} disabled={busy}
        helperText="Company registration or licence number, 1 to 50 characters" onChange={e => setRegistration(e.target.value)} />
      <TextField label="Notes" multiline minRows={2} value={notes} disabled={busy} helperText="Optional, up to 1000 characters"
        onChange={e => setNotes(e.target.value)} />
      {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
      <Button type="submit" variant="contained" disabled={busy || !ready}>Send request</Button>
    </Stack>}
    <Typography variant="h6" component="h2">My requests</Typography>
    <Button disabled={busy || list.loading} onClick={list.refresh}>Refresh requests</Button>
    {list.loading && <ResourceState loading error="" />}
    {!list.loading && list.error && <><Alert severity="error">{list.error}</Alert><Button onClick={list.refresh}>Try again</Button></>}
    {list.data && !list.data.requests?.length && <Typography>No requests on this page.</Typography>}
    {list.data?.requests?.map(r => <RequestCard key={r.requestId} request={r} />)}
    <Pager next={list.data?.pagination?.nextPageToken} token={token} busy={busy || list.loading} onPage={setToken} />
  </Stack>;
}
