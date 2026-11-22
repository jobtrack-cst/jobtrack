import { useState } from 'react';
import { Alert, Box, Button, Stack, Tab, Tabs, Typography } from '@mui/material';
import { CompanyAvatar } from '../../components/CompanyAvatar';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { ListPanel, ListRow, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { TextInput } from '../../components/TextInput';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { StatusChip } from '../../components/StatusChip';
import { gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { approveVerificationRequest, listVerificationRequests, rejectVerificationRequest, type RequestStatus, type VerificationRequest } from './api';
import { CompanyDetails } from './CompanyDetails';

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

function RequestCard({ request, busy, onDecide }: { request: VerificationRequest; busy: boolean; onDecide: (approve: boolean) => void }) {
  const decided = request.status === 'REQUEST_STATUS_APPROVED' || request.status === 'REQUEST_STATUS_REJECTED';
  return <ListRow>
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }}>
        <CompanyAvatar employerId={request.employerId} showName />
        <StatusChip kind="request" status={request.status} />
      </Stack>
      <Box>
        <Typography variant="h3" component="h2">Registration number: {request.registrationNumber || 'Not set'}</Typography>
        <Typography variant="body2" color="text.secondary">
          Submitted {shortDate(request.timestamps?.createdAt) || 'date unknown'}
          {decided && ` · Decided ${shortDate(request.timestamps?.updatedAt) || 'date unknown'} by an admin`}
        </Typography>
      </Box>
      {request.notes && <Typography sx={{ whiteSpace: 'pre-wrap' }}>Notes: {request.notes}</Typography>}
      {request.status === 'REQUEST_STATUS_REJECTED' && <Typography sx={{ whiteSpace: 'pre-wrap' }}>Reason: {request.rejectionReason || 'Not given'}</Typography>}
      <CompanyDetails employerId={request.employerId} />
      {request.status === 'REQUEST_STATUS_PENDING' && <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', pt: 0.5 }}>
        <Button variant="contained" disabled={busy} onClick={() => onDecide(true)}>Approve</Button>
        <Button variant="outlined" color="error" disabled={busy} onClick={() => onDecide(false)}>Reject</Button>
      </Stack>}
    </Stack>
  </ListRow>;
}

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
  const requests = list.data?.requests ?? [];
  const total = Number(list.data?.pagination?.totalCount ?? requests.length);
  return <Stack spacing={4}>
    <PageHeader title="Verification" description="Employer registrations, oldest first. Approving lets the employer publish jobs; a decision cannot be changed."
      action={<Button variant="outlined" disabled={busy || list.loading} onClick={list.refresh}>Refresh requests</Button>} />
    <Tabs value={filter} aria-label="Request status" variant="scrollable" allowScrollButtonsMobile
      onChange={(_, value: Filter) => { if (!busy) { setFilter(value); setToken(''); setNotice(''); } }}>
      {filters.map(([value, label]) => <Tab key={value} value={value} label={label} disabled={busy} />)}
    </Tabs>
    {notice && <Alert severity="success">{notice}</Alert>}
    {list.loading && <LoadingRows rows={3} height={140} label="Loading requests" />}
    {!list.loading && list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !requests.length && <EmptyState title="No requests here" description={filter === 'REQUEST_STATUS_PENDING' ? 'No employer is waiting for a decision.' : 'No requests on this page.'} />}
    {!!requests.length && <Stack spacing={2}>
      <ResultsHeader title={`${total.toLocaleString()} ${total === 1 ? 'request' : 'requests'}`} note="Oldest first" />
      <ListPanel label="Verification requests">
        {requests.map(r => <RequestCard key={r.requestId} request={r} busy={busy} onDecide={approve => open(r, approve)} />)}
      </ListPanel>
    </Stack>}
    <Pager token={token} nextToken={list.data?.pagination?.nextPageToken} totalCount={list.data?.pagination?.totalCount}
      count={requests.length} busy={busy || list.loading} onPage={setToken} />
    <ConfirmDialog open={!!decision} busy={busy} error={error} destructive={!decision?.approve}
      title={decision?.approve ? 'Approve this verification request?' : 'Reject this verification request?'}
      confirmLabel={decision?.approve ? 'Confirm approval' : 'Confirm rejection'} confirmDisabled={!canConfirm}
      onConfirm={() => void decide()} onCancel={() => setDecision(undefined)}>
      <Typography>Registration number: {decision?.request.registrationNumber || 'Not set'}</Typography>
      {decision?.approve
        ? <Typography color="text.secondary">The employer becomes verified and can publish jobs.</Typography>
        : <TextInput label="Reason" required multiline minRows={2} value={reason} disabled={busy}
          helperText="Shown to the employer, 1 to 500 characters" onChange={e => setReason(e.target.value)} />}
      {!decision?.approve && reasonTooLong && <Alert severity="warning">The reason can have at most 500 characters.</Alert>}
    </ConfirmDialog>
  </Stack>;
}
