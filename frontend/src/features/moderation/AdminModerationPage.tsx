import { useState } from 'react';
import { Alert, Button, Card, CardContent, Chip, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, gatewayMessage, grpcCode } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { dismissReport, listReports, removeJob, type Report, type ReportStatus } from './api';
import { readyToSend, reasonProblems } from './rules';

type Filter = ReportStatus | 'ALL';
const filters: [Filter, string][] = [
  ['REPORT_STATUS_OPEN', 'Open'], ['REPORT_STATUS_DISMISSED', 'Dismissed'], ['REPORT_STATUS_ACTIONED', 'Actioned'], ['ALL', 'All'],
];
type Color = 'default' | 'warning' | 'error';
const views: Record<ReportStatus, { label: string; color: Color }> = {
  REPORT_STATUS_UNSPECIFIED: { label: 'Unknown', color: 'default' },
  REPORT_STATUS_OPEN: { label: 'Open', color: 'warning' },
  REPORT_STATUS_DISMISSED: { label: 'Dismissed', color: 'default' },
  REPORT_STATUS_ACTIONED: { label: 'Actioned', color: 'error' },
};
const listMessages = {
  [grpcCode.permissionDenied]: 'Only admins can view reports.',
  [grpcCode.invalidArgument]: 'The list could not be loaded with this filter.',
};
const dismissMessages = {
  [grpcCode.failedPrecondition]: 'This report is no longer open. Refresh the list to see the latest state.',
  [grpcCode.notFound]: 'This report was not found. Refresh the list.',
  [grpcCode.permissionDenied]: 'Only admins can dismiss reports.',
};
const removeMessages = {
  [grpcCode.failedPrecondition]: 'This job has already been removed.',
  [grpcCode.invalidArgument]: 'The job was not removed. Use a valid job ID and a reason of 1 to 500 characters.',
  [grpcCode.permissionDenied]: 'Only admins can remove jobs.',
};

export function AdminModerationPage() {
  const [filter, setFilter] = useState<Filter>('REPORT_STATUS_OPEN');
  const [token, setToken] = useState('');
  const list = useLoad(() => listReports(token, filter === 'ALL' ? undefined : filter), [token, filter], listMessages);
  const [dismissing, setDismissing] = useState<Report>();
  const [removing, setRemoving] = useState<{ fixedJob: boolean }>();
  const [jobId, setJobId] = useState('');
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [dialogError, setDialogError] = useState('');
  const [notice, setNotice] = useState('');
  const problems = reasonProblems(jobId, reason, 500);
  const canRemove = readyToSend(jobId, reason, problems);
  function openRemove(id = '') { setRemoving({ fixedJob: id !== '' }); setJobId(id); setReason(''); setDialogError(''); setNotice(''); }
  function openDismiss(report: Report) { setDismissing(report); setDialogError(''); setNotice(''); }
  async function run(action: () => Promise<unknown>, done: string, messages: Partial<Record<number, string>>, close: () => void) {
    if (busy) return;
    setBusy(true); setDialogError('');
    try { await action(); close(); setNotice(done); list.refresh(); }
    catch (failure) { setDialogError(gatewayMessage(failure, messages)); }
    finally { setBusy(false); }
  }
  const dismiss = () => dismissing && run(() => dismissReport(dismissing.reportId), 'Report dismissed. The job stays published.',
    dismissMessages, () => setDismissing(undefined));
  const remove = () => canRemove && run(() => removeJob(jobId.trim().toLowerCase(), reason.trim()),
    'Job removed. Its open reports are marked as actioned, and the job closes for everyone shortly.', removeMessages, () => setRemoving(undefined));
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Job reports</Typography>
    <Typography>Reports are listed newest first. Dismiss a report to keep the job, or remove the job to close it for everyone.</Typography>
    <TextField select label="Status" value={filter} disabled={busy} sx={{ maxWidth: 240 }}
      onChange={e => { setFilter(e.target.value as Filter); setToken(''); setNotice(''); }}>
      {filters.map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
    </TextField>
    <Stack direction="row" spacing={1}>
      <Button disabled={busy || list.loading} onClick={list.refresh}>Refresh reports</Button>
      <Button color="error" disabled={busy} onClick={() => openRemove()}>Remove a job by ID</Button>
    </Stack>
    {notice && <Alert severity="success">{notice}</Alert>}
    {list.loading && <ResourceState loading error="" />}
    {!list.loading && list.error && <><Alert severity="error">{list.error}</Alert><Button onClick={list.refresh}>Try again</Button></>}
    {list.data && !list.data.reports?.length && <Typography>No reports on this page.</Typography>}
    {list.data?.reports?.map(r => {
      const view = views[r.status ?? 'REPORT_STATUS_UNSPECIFIED'] ?? views.REPORT_STATUS_UNSPECIFIED;
      return <Card key={r.reportId}><CardContent><Stack spacing={1}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
          <Typography variant="h6" component="h2">Job: {r.jobId}</Typography><Chip label={view.label} color={view.color} />
        </Stack>
        <Typography sx={{ whiteSpace: 'pre-wrap' }}>Reason: {r.reason || 'Not given'}</Typography>
        <Typography>Reported: {formatTime(r.timestamps?.createdAt)}</Typography>
        <Typography variant="body2">Reporter: {r.reporterId || 'Unknown'}</Typography>
        <Typography variant="body2">Report ID: {r.reportId}</Typography>
        {r.status === 'REPORT_STATUS_OPEN' && <Stack direction="row" spacing={1}>
          <Button disabled={busy} onClick={() => openDismiss(r)}>Dismiss</Button>
          <Button color="error" disabled={busy} onClick={() => openRemove(r.jobId)}>Remove job</Button>
        </Stack>}
      </Stack></CardContent></Card>;
    })}
    <Pager next={list.data?.pagination?.nextPageToken} token={token} busy={busy || list.loading} onPage={setToken} />
    <Dialog open={!!dismissing} onClose={() => { if (!busy) setDismissing(undefined); }}>
      <DialogTitle>Dismiss this report?</DialogTitle>
      <DialogContent><Stack spacing={2}>
        <Typography>The report is closed with no action, and the job stays published.</Typography>
        {dialogError && <Alert severity="error">{dialogError}</Alert>}
      </Stack></DialogContent>
      <DialogActions>
        <Button disabled={busy} onClick={() => setDismissing(undefined)}>Cancel</Button>
        <Button disabled={busy} onClick={() => void dismiss()}>Confirm dismissal</Button>
      </DialogActions>
    </Dialog>
    <Dialog open={!!removing} fullWidth onClose={() => { if (!busy) setRemoving(undefined); }}>
      <DialogTitle>Remove this job?</DialogTitle>
      <DialogContent><Stack spacing={2} sx={{ pt: 1 }}>
        <Typography>The job closes for everyone and all its open reports are marked as actioned. This cannot be undone.</Typography>
        <TextField label="Job ID" required value={jobId} disabled={busy || removing?.fixedJob} onChange={e => setJobId(e.target.value)} />
        <TextField label="Reason" required multiline minRows={2} value={reason} disabled={busy} helperText="1 to 500 characters"
          onChange={e => setReason(e.target.value)} />
        {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
        {dialogError && <Alert severity="error">{dialogError}</Alert>}
      </Stack></DialogContent>
      <DialogActions>
        <Button disabled={busy} onClick={() => setRemoving(undefined)}>Cancel</Button>
        <Button color="error" disabled={busy || !canRemove} onClick={() => void remove()}>Confirm removal</Button>
      </DialogActions>
    </Dialog>
  </Stack>;
}
