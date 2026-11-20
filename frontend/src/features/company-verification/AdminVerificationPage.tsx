import { EmptyState } from '../../components/EmptyState';
import { useState } from 'react';
import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { Pager, ResourceState } from '../application/shared';
import { gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { approveVerificationRequest, listVerificationRequests, rejectVerificationRequest, type RequestStatus, type VerificationRequest } from './api';
import { CompanyDetails } from './CompanyDetails';
import { RequestCard } from './RequestCard';

type Filter = RequestStatus | 'ALL';
const filters: [Filter, string][] = [
  ['REQUEST_STATUS_PENDING', 'Pending'], ['REQUEST_STATUS_APPROVED', 'Approved'], ['REQUEST_STATUS_REJECTED', 'Rejected'], ['ALL', 'All'],
];
const listMessages = {
  [grpcCode.permissionDenied]: 'Only admins can view verification requests.',
  [grpcCode.invalidArgument]: 'The list could not be loaded with this filter.',
};
const decideMessages = {
  [grpcCode.failedPrecondition]: 'This request has already been decided. Refresh the list to see the latest state.',
  [grpcCode.invalidArgument]: 'The reason must have 1 to 500 characters.',
  [grpcCode.notFound]: 'This request was not found. Refresh the list.',
  [grpcCode.permissionDenied]: 'Only admins can approve or reject verification requests.',
};

export function AdminVerificationPage() {
  const [filter, setFilter] = useState<Filter>('REQUEST_STATUS_PENDING');
  const [token, setToken] = useState('');
  const list = useLoad(() => listVerificationRequests(token, filter === 'ALL' ? undefined : filter), [token, filter], listMessages);
  const [decision, setDecision] = useState<{ request: VerificationRequest; approve: boolean }>();
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const reasonTooLong = length(reason) > 500;
  const canConfirm = !!decision && (decision.approve || (length(reason) > 0 && !reasonTooLong));
  function open(request: VerificationRequest, approve: boolean) {
    setDecision({ request, approve }); setReason(''); setError(''); setNotice('');
  }
  async function decide() {
    if (!decision || busy || !canConfirm) return;
    setBusy(true); setError('');
    try {
      if (decision.approve) await approveVerificationRequest(decision.request.requestId);
      else await rejectVerificationRequest(decision.request.requestId, reason.trim());
      setNotice(decision.approve
        ? 'Request approved. The employer profile shows Verified shortly.'
        : 'Request rejected. The employer sees the reason and can send a new request.');
      setDecision(undefined);
      list.refresh();
    } catch (failure) { setError(gatewayMessage(failure, decideMessages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Verification requests</Typography>
    <Typography>Requests are listed oldest first. Approving lets the employer publish jobs; rejecting sends your reason to the employer. A decision cannot be changed.</Typography>
    <TextField select label="Status" value={filter} disabled={busy} sx={{ maxWidth: 240 }}
      onChange={e => { setFilter(e.target.value as Filter); setToken(''); setNotice(''); }}>
      {filters.map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
    </TextField>
    <Button disabled={busy || list.loading} onClick={list.refresh}>Refresh requests</Button>
    {notice && <Alert severity="success">{notice}</Alert>}
    {list.loading && <ResourceState loading error="" />}
    {!list.loading && list.error && <><Alert severity="error">{list.error}</Alert><Button onClick={list.refresh}>Try again</Button></>}
    {list.data && !list.data.requests?.length && <EmptyState>No requests on this page.</EmptyState>}
    {list.data?.requests?.map(r => <RequestCard key={r.requestId} request={r} admin>
      <Typography variant="body2">Employer ID: {r.employerId}</Typography>
      <CompanyDetails employerId={r.employerId} />
      {r.status === 'REQUEST_STATUS_PENDING' && <Stack direction="row" spacing={1}>
        <Button variant="contained" color="success" disabled={busy} onClick={() => open(r, true)}>Approve</Button>
        <Button color="error" disabled={busy} onClick={() => open(r, false)}>Reject</Button>
      </Stack>}
    </RequestCard>)}
    <Pager next={list.data?.pagination?.nextPageToken} token={token} busy={busy || list.loading} onPage={setToken} />
    <Dialog open={!!decision} fullWidth onClose={() => { if (!busy) setDecision(undefined); }}>
      <DialogTitle>{decision?.approve ? 'Approve this verification request?' : 'Reject this verification request?'}</DialogTitle>
      <DialogContent><Stack spacing={2} sx={{ pt: 1 }}>
        <Typography>Registration number: {decision?.request.registrationNumber || 'Not set'}</Typography>
        {decision?.approve
          ? <Typography>The employer becomes verified and can publish jobs.</Typography>
          : <TextField label="Reason" required multiline minRows={2} value={reason} disabled={busy}
            helperText="Shown to the employer, 1 to 500 characters" onChange={e => setReason(e.target.value)} />}
        {!decision?.approve && reasonTooLong && <Alert severity="warning">The reason can have at most 500 characters.</Alert>}
        {error && <Alert severity="error">{error}</Alert>}
      </Stack></DialogContent>
      <DialogActions>
        <Button disabled={busy} onClick={() => setDecision(undefined)}>Cancel</Button>
        <Button variant="contained" color={decision?.approve ? 'success' : 'error'} disabled={busy || !canConfirm} onClick={() => void decide()}>{decision?.approve ? 'Confirm approval' : 'Confirm rejection'}</Button>
      </DialogActions>
    </Dialog>
  </Stack>;
}
