import { useState } from 'react';
import { Alert, Box, Button, FormControlLabel, Link, Radio, RadioGroup, Stack, Typography } from '@mui/material';
import { Link as RouterLink, useParams } from 'react-router';
import { Card } from '../../components/Card';
import { LogoSquare, useCompany } from '../../components/CompanyAvatar';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DeadlineTag } from '../../components/DeadlineTag';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { TagChip } from '../../components/JobCard';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { rememberJob } from '../../components/useJob';
import { TextInput } from '../../components/TextInput';
import { tokens } from '../../theme';
import { getJob, submitApplication } from '../application/api';
import { allPages, findApplicationForJob } from '../application/applicant';
import { errorMessage, validId } from '../application/shared';
import { gatewayMessage, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { alertReferences } from '../job-alert/api';
import type { Job } from '../job-posting/api';
import { reportJob } from '../moderation/api';
import { readyToSend, reasonProblems } from '../moderation/rules';
import { listMyResumes } from '../resume/api';
import { SaveJobButton, useSaveJob } from '../saved-jobs/SaveJobButton';

const back = { to: '/applicant/jobs', label: 'Find jobs' };
const closedMessage = 'This job is closed, unavailable for applications, or past its deadline.';
const reportMessages = {
  [grpcCode.invalidArgument]: 'The report was not accepted. Use a valid job ID and a reason of 1 to 1000 characters.',
  [grpcCode.alreadyExists]: 'You already have an open report for this job. An admin will review it.',
  [grpcCode.failedPrecondition]: 'This job has already been removed, so it cannot be reported.',
};
const maxReason = 1000;

export function JobDetailsPage() {
  const { jobId = '' } = useParams();
  // An address that is not a job ID never reaches the gateway.
  if (!validId(jobId)) return <Stack spacing={4}>
    <PageHeader back={back} title="Job not found" />
    <EmptyState title="This job does not exist" description="The link may be incomplete. Find the job again from the search."
      action={<Button component={RouterLink} to="/applicant/jobs" variant="outlined">Find jobs</Button>} />
  </Stack>;
  return <JobDetails key={jobId.toLowerCase()} jobId={jobId.toLowerCase()} />;
}

function JobDetails({ jobId }: { jobId: string }) {
  const resource = useLoad(async () => { const job = (await getJob(jobId)).job as Job; rememberJob(job); return job; }, [jobId], {});
  // Optional: tag and category names. Without them the tags are left out.
  const refs = useLoad(alertReferences, [], {});
  const existing = useLoad(() => findApplicationForJob(jobId), [jobId], {});
  const save = useSaveJob(jobId);
  const [applying, setApplying] = useState(false);
  const [reporting, setReporting] = useState(false);
  const [notice, setNotice] = useState('');
  const job = resource.data;
  const { company } = useCompany(job?.employerId);
  if (resource.loading) return <Stack spacing={4}><PageHeader back={back} title="Loading job…" /><LoadingRows rows={1} height={240} label="Loading job" /></Stack>;
  if (!job) return <Stack spacing={4}><PageHeader back={back} title="Job details" />
    <ErrorState message={resource.error || 'This job is not available.'} onRetry={resource.refresh} /></Stack>;
  const open = job.status === 'JOB_STATUS_PUBLISHED' && Date.parse(job.deadline) > Date.now();
  const meta = [company?.name, job.location].filter(Boolean).join(', ');
  const tags = (job.tagIds ?? []).map(id => refs.data?.tags.find(t => t.tagId === id)?.name).filter((n): n is string => !!n);
  const category = refs.data?.categories.find(c => c.categoryId === job.categoryId)?.name;
  const mine = existing.data;
  const applyAction = mine
    ? <Button component={RouterLink} to={`/applicant/applications/${encodeURIComponent(mine.app.applicationId)}`} variant="contained">View application</Button>
    : <Button variant="contained" disabled={!open || existing.loading} onClick={() => { setNotice(''); setApplying(true); }}>Apply</Button>;
  return <Stack spacing={4}>
    <PageHeader back={back} title={job.title || 'Untitled job'} description={meta || undefined} action={applyAction} />
    {notice && <Alert severity="success" role="status">{notice}</Alert>}
    {!open && <Alert severity="warning">{closedMessage}</Alert>}
    {mine && <Alert severity="info" icon={false}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        {mine.active ? <ToneChip label="Applied" tone="accent" /> : <StatusChip kind="application" status={mine.status} />}
        <span>You applied{shortDate(mine.app.timestamps?.createdAt) && ` on ${shortDate(mine.app.timestamps?.createdAt)}`}. Follow it on its application page.</span>
      </Stack>
    </Alert>}
    {existing.error && <ErrorState message={existing.error} onRetry={existing.refresh} />}
    <Card component="section">
      <Stack spacing={3}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1.5 }}>
          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
            <LogoSquare src={company?.url} name={company?.name} size={48} />
            <Box sx={{ minWidth: 0 }}>
              <Typography sx={{ fontWeight: 700 }}>{company?.name || 'Company'}</Typography>
              <Typography variant="body2" color="text.secondary">{job.location || 'Location not given'}</Typography>
            </Box>
          </Stack>
          <Stack direction="row" spacing={1}>
            {job.status !== 'JOB_STATUS_PUBLISHED' && <StatusChip kind="job" status={job.status} />}
            <DeadlineTag deadline={job.deadline} />
          </Stack>
        </Stack>
        {(category || !!tags.length) && <Stack direction="row" aria-label="Tags" sx={{ flexWrap: 'wrap', gap: 1 }}>
          {category && <TagChip label={category} />}{tags.map(t => <TagChip key={t} label={t} />)}
        </Stack>}
        <Box>
          <Typography variant="h2" component="h2" sx={{ mb: 1 }}>About the role</Typography>
          <Typography sx={{ whiteSpace: 'pre-wrap', color: tokens.textSecondary }}>{job.description || 'No description given.'}</Typography>
        </Box>
        <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', rowGap: 1, alignItems: 'center', pt: 1, borderTop: `1px solid ${tokens.border}` }}>
          <SaveJobButton state={save} disabled={!open && !save.saved} />
          <Button color="error" onClick={() => { setNotice(''); setReporting(true); }}>Report this job</Button>
        </Stack>
        {save.error && <Alert severity="error">{save.error}</Alert>}
        {save.notice && <Typography variant="body2" role="status" color="text.secondary">{save.notice}</Typography>}
      </Stack>
    </Card>
    {applying && <ApplyDialog job={job} onClose={() => setApplying(false)}
      onApplied={() => { setApplying(false); setNotice('Application sent. You can follow it on its application page.'); existing.refresh(); }} />}
    {reporting && <ReportDialog jobId={jobId} onClose={() => setReporting(false)}
      onSent={() => { setReporting(false); setNotice('Report sent. An admin will review it.'); }} />}
  </Stack>;
}

function ApplyDialog({ job, onClose, onApplied }: { job: Job; onClose: () => void; onApplied: () => void }) {
  // Every page, so a validated resume on a later page can still be chosen.
  const resumes = useLoad(() => allPages(token => listMyResumes(token).then(r => ({ items: r.resumes, pagination: r.pagination }))), [], {});
  const [resumeId, setResumeId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const validated = resumes.data?.filter(r => r.validationStatus === 'RESUME_VALIDATION_STATUS_VALIDATED') ?? [];
  const checking = resumes.data?.filter(r => r.validationStatus === 'RESUME_VALIDATION_STATUS_PENDING').length ?? 0;
  const open = job.status === 'JOB_STATUS_PUBLISHED' && Date.parse(job.deadline) > Date.now();
  async function apply() {
    if (busy || !open || !validated.some(r => r.resumeId === resumeId)) return;
    setBusy(true); setError('');
    try { await submitApplication(job.jobId, resumeId); onApplied(); }
    catch (reason) { setError(errorMessage(reason, 'apply')); setBusy(false); }
  }
  return <ConfirmDialog open title={`Apply for ${job.title || 'this job'}`} confirmLabel="Send application" busy={busy} error={error}
    confirmDisabled={!open || !validated.some(r => r.resumeId === resumeId)} onCancel={onClose} onConfirm={() => void apply()}>
    {!open && <Alert severity="warning">{closedMessage}</Alert>}
    {resumes.loading && <LoadingRows rows={2} label="Loading resumes" />}
    {resumes.error && <ErrorState message={resumes.error} onRetry={resumes.refresh} />}
    {resumes.data && !validated.length && <Stack spacing={1}>
      <Typography>You have no validated resume yet. Upload a PDF; it can be used once it has been checked.</Typography>
      <Link component={RouterLink} to="/applicant/profile?tab=resumes" sx={{ alignSelf: 'flex-start', minHeight: 44, display: 'inline-flex', alignItems: 'center' }}>Upload a resume</Link>
    </Stack>}
    {!!validated.length && <RadioGroup aria-label="Resume" value={resumeId} onChange={e => setResumeId(e.target.value)}>
      <Typography sx={{ fontWeight: 600, mb: 0.5 }}>Choose a validated resume</Typography>
      {validated.map(r => <FormControlLabel key={r.resumeId} value={r.resumeId} disabled={busy} control={<Radio />}
        label={<span>{r.fileName}{shortDate(r.timestamps?.createdAt) && <Typography component="span" variant="body2" color="text.secondary"> · uploaded {shortDate(r.timestamps?.createdAt)}</Typography>}</span>} />)}
    </RadioGroup>}
    {checking > 0 && <Typography variant="body2" color="text.secondary">{checking === 1 ? 'One resume is' : `${checking} resumes are`} still being checked.</Typography>}
  </ConfirmDialog>;
}

function ReportDialog({ jobId, onClose, onSent }: { jobId: string; onClose: () => void; onSent: () => void }) {
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const problems = reasonProblems(jobId, reason, maxReason);
  const ready = readyToSend(jobId, reason, problems);
  async function send() {
    if (busy || !ready) return;
    setBusy(true); setError('');
    try { await reportJob(jobId, reason.trim()); onSent(); }
    catch (reason) { setError(gatewayMessage(reason, reportMessages)); setBusy(false); }
  }
  return <ConfirmDialog open title="Report this job" confirmLabel="Send report" destructive busy={busy} error={error}
    confirmDisabled={!ready} onCancel={onClose} onConfirm={() => void send()}>
    <Typography color="text.secondary">Report a posting that looks false, misleading or inappropriate. An admin reviews every report and can remove the job.</Typography>
    <TextInput label="Reason" required multiline minRows={4} value={reason} disabled={busy} onChange={e => setReason(e.target.value)}
      helperText={`1 to ${maxReason} characters. ${length(reason)} of ${maxReason}.`} error={problems.length > 0} />
    {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
  </ConfirmDialog>;
}
