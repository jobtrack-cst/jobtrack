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
import { jobName, useJob } from '../../components/useJob';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { dismissReport, listReports, removeJob, type Report, type ReportStatus } from './api';
import { readyToSend, reasonProblems } from './rules';

type Filter = ReportStatus | 'ALL';
const filters: [Filter, string][] = [
  ['REPORT_STATUS_OPEN', 'Open'], ['REPORT_STATUS_DISMISSED', 'Dismissed'], ['REPORT_STATUS_ACTIONED', 'Job removed'], ['ALL', 'All'],
];
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

// The job being removed, by name. Read live, so a title still loading when the dialog opened appears once it arrives.
function RemovingJob({ jobId }: { jobId: string }) {
  const job = useJob(jobId);
  return <Typography variant="subtitle1">Job: {jobName(job)}</Typography>;
}

function ReportCard({ report, busy, onDismiss, onRemove }: { report: Report; busy: boolean; onDismiss: () => void; onRemove: () => void }) {
  const job = useJob(report.jobId);
  const title = jobName(job);
  return <ListRow>
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap' }}>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="h3" component="h2">{title}</Typography>
          {job.job?.employerId && <Box sx={{ mt: 1 }}><CompanyAvatar employerId={job.job.employerId} size={32} showName /></Box>}
        </Box>
        <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}>
          {job.job?.status === 'JOB_STATUS_REMOVED' && <StatusChip kind="job" status="JOB_STATUS_REMOVED" />}
          <StatusChip kind="report" status={report.status} />
        </Stack>
      </Stack>
      <Typography sx={{ whiteSpace: 'pre-wrap' }}>Reason: {report.reason || 'Not given'}</Typography>
      <Typography variant="body2" color="text.secondary">Reported {shortDate(report.timestamps?.createdAt) || 'date unknown'}</Typography>
      {report.status === 'REPORT_STATUS_OPEN' && <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', pt: 0.5 }}>
        <Button variant="outlined" disabled={busy} onClick={onDismiss}>Dismiss</Button>
        <Button variant="outlined" color="error" disabled={busy} onClick={onRemove}>Remove job</Button>
      </Stack>}
    </Stack>
  </ListRow>;
}

export function AdminModerationPage() {
  const [filter, setFilter] = useState<Filter>('REPORT_STATUS_OPEN');
  const [token, setToken] = useState('');
  const list = useLoad(() => listReports(token, filter === 'ALL' ? undefined : filter), [token, filter], listMessages);
  const [dismissing, setDismissing] = useState<Report>();
  // fromReport: the job comes from a report and is shown by name; otherwise the admin types its ID.
  const [removing, setRemoving] = useState<{ fromReport: boolean }>();
  const [jobId, setJobId] = useState('');
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [dialogError, setDialogError] = useState('');
  const [notice, setNotice] = useState('');
  const problems = reasonProblems(jobId, reason, 500);
  const canRemove = readyToSend(jobId, reason, problems);
  function openRemove(id = '') { setRemoving({ fromReport: id !== '' }); setJobId(id); setReason(''); setDialogError(''); setNotice(''); }
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
  const reports = list.data?.reports ?? [];
  const total = Number(list.data?.pagination?.totalCount ?? reports.length);
  return <Stack spacing={4}>
    <PageHeader title="Moderation" description="Job reports, newest first. Dismiss a report to keep the job, or remove the job to close it for everyone."
      action={<Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}>
        <Button variant="outlined" disabled={busy || list.loading} onClick={list.refresh}>Refresh reports</Button>
        <Button variant="outlined" color="error" disabled={busy} onClick={() => openRemove()}>Remove a job by ID</Button>
      </Stack>} />
    <Tabs value={filter} aria-label="Report status" variant="scrollable" allowScrollButtonsMobile
      onChange={(_, value: Filter) => { if (!busy) { setFilter(value); setToken(''); setNotice(''); } }}>
      {filters.map(([value, label]) => <Tab key={value} value={value} label={label} disabled={busy} />)}
    </Tabs>
    {notice && <Alert severity="success">{notice}</Alert>}
    {list.loading && <LoadingRows rows={3} height={120} label="Loading reports" />}
    {!list.loading && list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !reports.length && <EmptyState title="No reports here" description={filter === 'REPORT_STATUS_OPEN' ? 'No job report is waiting for review.' : 'No reports on this page.'} />}
    {!!reports.length && <Stack spacing={2}>
      <ResultsHeader title={`${total.toLocaleString()} ${total === 1 ? 'report' : 'reports'}`} note="Newest first" />
      <ListPanel label="Job reports">
        {reports.map(r => <ReportCard key={r.reportId} report={r} busy={busy} onDismiss={() => openDismiss(r)} onRemove={() => openRemove(r.jobId)} />)}
      </ListPanel>
    </Stack>}
    <Pager token={token} nextToken={list.data?.pagination?.nextPageToken} totalCount={list.data?.pagination?.totalCount}
      count={reports.length} busy={busy || list.loading} onPage={setToken} />
    <ConfirmDialog open={!!dismissing} busy={busy} error={dialogError} title="Dismiss this report?" confirmLabel="Confirm dismissal"
      onConfirm={() => void dismiss()} onCancel={() => setDismissing(undefined)}>
      <Typography>The report is closed with no action, and the job stays published.</Typography>
    </ConfirmDialog>
    <ConfirmDialog open={!!removing} busy={busy} error={dialogError} destructive title="Remove this job?" confirmLabel="Confirm removal"
      confirmDisabled={!canRemove} onConfirm={() => void remove()} onCancel={() => setRemoving(undefined)}>
      <Typography>The job closes for everyone and all its open reports are marked as actioned. This cannot be undone.</Typography>
      {removing?.fromReport
        ? <RemovingJob jobId={jobId} />
        // Admins only: removing a job that has no report needs its ID (redesign doc Section 5).
        : <TextInput label="Job ID" required value={jobId} disabled={busy} onChange={e => setJobId(e.target.value)} />}
      <TextInput label="Reason" required multiline minRows={2} value={reason} disabled={busy} helperText="1 to 500 characters"
        onChange={e => setReason(e.target.value)} />
      {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
    </ConfirmDialog>
  </Stack>;
}
