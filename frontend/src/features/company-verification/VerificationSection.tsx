import { useEffect, useState } from 'react';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { ListPanel, ListRow } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { Pager } from '../../components/Pager';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { TextInput } from '../../components/TextInput';
import { gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import type { CompanyProfile } from '../employer/api';
import { listMyVerificationRequests, submitVerificationRequest, type VerificationRequest } from './api';

const listMessages = { [grpcCode.permissionDenied]: 'Only employers can view verification requests.' };
const submitMessages = {
  [grpcCode.invalidArgument]: 'The request was not accepted. The registration number needs 1 to 50 characters and notes at most 1000.',
  [grpcCode.alreadyExists]: 'You already have a pending request. Wait for the admin decision before sending another.',
  [grpcCode.failedPrecondition]: 'Your company is already verified, so no new request is needed.',
  [grpcCode.permissionDenied]: 'Only employers can request verification.',
};

function RequestRow({ request }: { request: VerificationRequest }) {
  const decided = request.status === 'REQUEST_STATUS_APPROVED' || request.status === 'REQUEST_STATUS_REJECTED';
  const sent = shortDate(request.timestamps?.createdAt);
  const decidedOn = decided ? shortDate(request.timestamps?.updatedAt) : '';
  return <ListRow>
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ justifyContent: 'space-between', alignItems: { sm: 'flex-start' } }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography sx={{ fontWeight: 700 }}>Registration number: {request.registrationNumber || 'Not set'}</Typography>
        <Typography variant="body2" color="text.secondary">
          {[sent && `Sent ${sent}`, decidedOn && `Decided ${decidedOn}`].filter(Boolean).join('. ') || 'Date unknown'}
        </Typography>
        {request.notes && <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: 'pre-wrap' }}>Notes: {request.notes}</Typography>}
        {request.status === 'REQUEST_STATUS_REJECTED' && <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: 'pre-wrap' }}>
          Reason: {request.rejectionReason || 'Not given'}</Typography>}
      </Box>
      <Box sx={{ flexShrink: 0 }}><StatusChip kind="request" status={request.status} /></Box>
    </Stack>
  </ListRow>;
}

// Verification on the Company page: the profile's status and rejection reason, the
// request form, and the employer's own requests (newest first, JT-33). The status
// comes from the profile, which verification events update a moment after a decision.
export function VerificationSection({ profile, onRefreshProfile }: { profile: CompanyProfile; onRefreshProfile: () => void }) {
  const [token, setToken] = useState('');
  const list = useLoad(() => listMyVerificationRequests(token), [token], listMessages);
  // The newest request, known whenever the first page was read.
  const [latest, setLatest] = useState<VerificationRequest | null>();
  useEffect(() => { if (token === '' && list.data) setLatest(list.data.requests?.[0] ?? null); }, [token, list.data]);
  const [registration, setRegistration] = useState('');
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const status = profile.verificationStatus;
  const verified = status === 'VERIFICATION_STATUS_VERIFIED';
  const rejected = status === 'VERIFICATION_STATUS_REJECTED';
  const pending = latest?.status === 'REQUEST_STATUS_PENDING';
  const approved = latest?.status === 'REQUEST_STATUS_APPROVED';
  const canRequest = latest !== undefined && !verified && !pending && !approved;
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
      setNotice('Request sent. An admin will approve or reject it, and this page shows the result.');
      if (token) setToken(''); else list.refresh();
    } catch (reason) { setError(gatewayMessage(reason, submitMessages)); }
    finally { setBusy(false); }
  }
  function refresh() { setNotice(''); setError(''); onRefreshProfile(); if (token) setToken(''); else list.refresh(); }
  const line = verified ? 'Your company is verified. You can publish jobs.'
    : pending ? 'An admin is checking your registration. You can prepare draft jobs meanwhile.'
    : approved ? 'Your request was approved. The status here updates in a moment.'
    : rejected ? 'Verification was not approved. Read the reason, then send a new request.'
    : 'An admin checks your company registration before you can publish jobs.';
  const requests = list.data?.requests ?? [];
  return <Card component="section" label="Verification" sx={{ p: { xs: 2.5, sm: 4 } }}>
    <Stack spacing={3}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1 }}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
          <Typography variant="h2" component="h2">Verification</Typography>
          {pending && !verified ? <ToneChip label="Pending" tone="accent" /> : <StatusChip kind="verification" status={status} />}
        </Stack>
        <Button disabled={busy || list.loading} onClick={refresh}>Refresh</Button>
      </Stack>
      <Typography color="text.secondary">{line}</Typography>
      {rejected && !pending && <Alert severity="warning" sx={{ whiteSpace: 'pre-wrap' }}>Reason: {profile.rejectionReason || 'Not given'}</Alert>}
      {notice && <Alert severity="success" role="status">{notice}</Alert>}
      {canRequest && <Stack component="form" spacing={2} noValidate aria-label="Request verification"
        onSubmit={e => { e.preventDefault(); void submit(); }}>
        <Typography variant="h3" component="h3">{latest?.status === 'REQUEST_STATUS_REJECTED' ? 'Send a new request' : 'Request verification'}</Typography>
        <TextInput label="Registration number" required value={registration} disabled={busy}
          helperText={`Company registration or licence number. ${length(registration)} of 50 characters.`} onChange={e => setRegistration(e.target.value)} />
        <TextInput label="Notes" multiline minRows={2} value={notes} disabled={busy}
          helperText={`Optional. ${length(notes)} of 1000 characters.`} onChange={e => setNotes(e.target.value)} />
        {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
        {error && <Alert severity="error">{error}</Alert>}
        <Button type="submit" variant="contained" disabled={busy || !ready} sx={{ alignSelf: 'flex-start' }}>{busy ? 'Sending…' : 'Send request'}</Button>
      </Stack>}
      <Stack spacing={1.5}>
        <Typography variant="h3" component="h3">Your requests</Typography>
        {list.loading && <LoadingRows rows={2} label="Loading requests" />}
        {!list.loading && list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
        {list.data && !requests.length && <EmptyState description={token ? 'No requests on this page.' : 'You have not sent a request yet.'} />}
        {!!requests.length && <ListPanel label="Verification requests">{requests.map(r => <RequestRow key={r.requestId} request={r} />)}</ListPanel>}
        {list.data && <Pager token={token} nextToken={list.data.pagination?.nextPageToken} totalCount={list.data.pagination?.totalCount}
          count={requests.length} busy={busy || list.loading} onPage={setToken} />}
      </Stack>
    </Stack>
  </Card>;
}
