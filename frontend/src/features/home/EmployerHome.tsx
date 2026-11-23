import { Box, Button, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Card } from '../../components/Card';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { JobCard } from '../../components/JobCard';
import { CardGrid, ListPanel, ListRow } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { jobName, rememberJob, useJob } from '../../components/useJob';
import { tokens } from '../../theme';
import { ApplicantAvatar } from '../applicant-profile/ApplicantAvatar';
import { listApplications, type Application } from '../application/api';
import { statusOf } from '../application/applicant';
import { applicantLabel } from '../application/employer';
import { isNotFound } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listMyVerificationRequests } from '../company-verification/api';
import { getMyCompanyProfile } from '../employer/api';
import { listJobs, type Job } from '../job-posting/api';
import { ApplicationCount } from '../job-posting/ApplicationCount';
import { Section } from './Section';

const shown = 5;

async function loadVerification() {
  const profile = await getMyCompanyProfile().then(r => r.profile ?? null, (error: unknown) => { if (isNotFound(error)) return null; throw error; });
  if (!profile || profile.verificationStatus === 'VERIFICATION_STATUS_VERIFIED') return { profile, pending: false };
  // Requests are newest first, so the first one is the latest.
  const { requests = [] } = await listMyVerificationRequests('');
  return { profile, pending: requests[0]?.status === 'REQUEST_STATUS_PENDING' };
}

function VerificationCard() {
  const state = useLoad(loadVerification, [], {});
  if (state.loading) return <LoadingRows rows={1} label="Loading verification" />;
  if (state.error) return <ErrorState message={state.error} onRetry={state.refresh} />;
  const { profile, pending } = state.data!;
  const status = profile?.verificationStatus;
  const [title, line, next] = !profile
    ? ['Create your company profile', 'Applicants see it on your jobs, and you need it before verification.', ['/employer/company', 'Create profile']]
    : status === 'VERIFICATION_STATUS_VERIFIED' ? ['Your company is verified', 'You can publish jobs.', undefined]
    : pending ? ['Verification pending', 'An admin is checking your registration. You can prepare draft jobs meanwhile.', ['/employer/company', 'View request']]
    : status === 'VERIFICATION_STATUS_REJECTED' ? ['Verification was not approved', 'Read the reason, then send a new request.', ['/employer/company', 'Send a new request']]
    : ['Verify your company', 'An admin checks your registration before you can publish jobs.', ['/employer/company', 'Request verification']];
  const verified = status === 'VERIFICATION_STATUS_VERIFIED';
  return <Card sx={{ borderLeft: `4px solid ${verified ? tokens.success : tokens.accent}` }}>
    <Stack spacing={1} sx={{ alignItems: 'flex-start' }}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <Typography variant="h3" component="h2">{title}</Typography>
        {profile && (pending ? <ToneChip label="Pending" tone="accent" /> : <StatusChip kind="verification" status={status} />)}
      </Stack>
      <Typography color="text.secondary">{line}</Typography>
      {!pending && status === 'VERIFICATION_STATUS_REJECTED' && profile?.rejectionReason &&
        <Typography sx={{ whiteSpace: 'pre-wrap' }}>Reason: {profile.rejectionReason}</Typography>}
      {next && <Button component={Link} to={next[0]} variant="outlined" sx={{ mt: 1 }}>{next[1]}</Button>}
    </Stack>
  </Card>;
}

function OpenJobCard({ job }: { job: Job }) {
  return <JobCard job={job} viewTo={`/employer/jobs/${encodeURIComponent(job.jobId)}`} extra={<ApplicationCount jobId={job.jobId} />} />;
}

// A compact row: photo or initials, the applicant's name ("Applicant" before JT-114), the job
// and applied date, and the status; the whole row opens the application page.
function ApplicantRow({ app }: { app: Application }) {
  const job = useJob(app.jobId);
  // Same rule as the Applicants cards: withdrawn counts at once, no record yet is Submitted.
  const status = useLoad(() => statusOf(app), [app.applicationId, app.withdrawn], {});
  const name = applicantLabel(app);
  const applied = shortDate(app.timestamps?.createdAt);
  return <ListRow to={`/employer/applicants/${encodeURIComponent(app.applicationId)}`}>
    <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
      <ApplicantAvatar applicationId={app.applicationId} name={name} size={44} />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography variant="subtitle1" component="h3">{name}</Typography>
        <Typography variant="body2" color="text.secondary">{applied ? `${jobName(job)}, applied ${applied}` : jobName(job)}</Typography>
      </Box>
      {status.data && <Box sx={{ flexShrink: 0 }}><StatusChip kind="application" status={status.data} /></Box>}
    </Stack>
  </ListRow>;
}

export function EmployerHome() {
  const jobs = useLoad(async () => {
    const page = await listJobs('', 'JOB_STATUS_PUBLISHED');
    page.jobs?.forEach(rememberJob);
    return page;
  }, [], {});
  const applicants = useLoad(() => listApplications(''), [], {});
  const openJobs = jobs.data?.jobs ?? [];
  const newest = applicants.data?.applications ?? [];
  return <Stack spacing={4}>
    <PageHeader title="Home" description="Your verification, open jobs and newest applicants."
      action={<Button component={Link} to="/employer/jobs/new" variant="contained">Post a job</Button>} />
    <VerificationCard />
    <Section title="Open jobs" seeAll={['/employer/jobs', 'All jobs']}>
        {jobs.loading && <LoadingRows rows={2} label="Loading open jobs" />}
        {jobs.error && <ErrorState message={jobs.error} onRetry={jobs.refresh} />}
        {jobs.data && !openJobs.length && <EmptyState title="No open jobs" description="Published jobs that accept applications show here." />}
        {!!openJobs.length && <CardGrid>{openJobs.slice(0, shown).map(job => <OpenJobCard key={job.jobId} job={job} />)}</CardGrid>}
    </Section>
    <Section title="Newest applicants" seeAll={['/employer/applicants', 'All applicants']}>
        {applicants.loading && <LoadingRows rows={2} label="Loading applicants" />}
        {applicants.error && <ErrorState message={applicants.error} onRetry={applicants.refresh} />}
        {applicants.data && !newest.length && <EmptyState title="No applicants yet" description="Applications to your jobs show here, newest first." />}
        {!!newest.length && <ListPanel>{newest.slice(0, shown).map(app => <ApplicantRow key={app.applicationId} app={app} />)}</ListPanel>}
    </Section>
  </Stack>;
}
