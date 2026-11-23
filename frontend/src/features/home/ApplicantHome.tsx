import { Box, Button, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Card } from '../../components/Card';
import { CompanyAvatar, useCompany } from '../../components/CompanyAvatar';
import { closesWithin } from '../../components/DeadlineTag';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { firstName, shortDate } from '../../components/format';
import { JobCard } from '../../components/JobCard';
import { CardGrid } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { StatusRail } from '../../components/StatusRail';
import { jobName, loadJob, useJob } from '../../components/useJob';
import { tokens } from '../../theme';
import { getMyProfile } from '../applicant-profile/api';
import type { Application } from '../application/api';
import { loadMyApplications } from '../application/applicant';
import { isNotFound } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { useAuth } from '../auth/AuthProvider';
import type { Job } from '../job-posting/api';
import { listMyResumes } from '../resume/api';
import { listSavedJobs } from '../saved-jobs/api';
import { Section } from './Section';

const shown = 5;
const orNullIfMissing = <T,>(request: Promise<T>) => request.then(v => v, (error: unknown) => { if (isNotFound(error)) return null; throw error; });

// The newest application per job, still in progress (same rule as My applications).
const loadActiveApplications = async () => (await loadMyApplications()).filter(x => x.active);

async function loadClosingSoon() {
  const { savedJobs = [] } = await listSavedJobs('');
  // A job that cannot be read is skipped here; Saved shows every bookmark.
  const jobs = await Promise.all(savedJobs.map(s => loadJob(s.jobId).catch(() => null)));
  return jobs.filter((j): j is Job => !!j && j.status === 'JOB_STATUS_PUBLISHED' && closesWithin(j.deadline, 7))
    .sort((a, b) => Date.parse(a.deadline) - Date.parse(b.deadline));
}

async function loadReadiness() {
  const [profile, resumes] = await Promise.all([orNullIfMissing(getMyProfile()), listMyResumes('')]);
  return { hasProfile: profile !== null, hasValidatedResume: !!resumes.resumes?.some(r => r.validationStatus === 'RESUME_VALIDATION_STATUS_VALIDATED') };
}

function ApplicationCard({ app, status }: { app: Application; status: string }) {
  const job = useJob(app.jobId);
  const { company } = useCompany(app.employerId);
  const applied = shortDate(app.timestamps?.createdAt);
  return <Card to={`/applicant/applications/${encodeURIComponent(app.applicationId)}`} component="li">
    <Stack spacing={2}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
        <CompanyAvatar employerId={app.employerId} size={48} />
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="h3" component="h3">{jobName(job)}</Typography>
          <Typography variant="body2" color="text.secondary">{[company?.name, applied && `Applied ${applied}`].filter(Boolean).join(' · ')}</Typography>
        </Box>
      </Stack>
      <StatusRail status={status} />
    </Stack>
  </Card>;
}


function ReadinessHint() {
  const readiness = useLoad(loadReadiness, [], {});
  const r = readiness.data;
  // Quiet on failure: the hint is optional and the other sections show errors.
  if (!r || (r.hasProfile && r.hasValidatedResume)) return null;
  const [title, line, to, label] = !r.hasProfile
    ? ['Create your profile', 'Employers read your summary, education and skills when you apply.', '/applicant/profile', 'Create profile']
    : ['Upload a resume', 'You need a validated PDF resume before you can apply for a job.', '/applicant/profile?tab=resumes', 'Upload resume'];
  return <Card sx={{ borderLeft: `4px solid ${tokens.accent}` }}>
    <Stack spacing={1} sx={{ alignItems: 'flex-start' }}>
      <Typography variant="h3" component="h2">{title}</Typography>
      <Typography color="text.secondary">{line}</Typography>
      <Button component={Link} to={to} variant="outlined" sx={{ mt: 1 }}>{label}</Button>
    </Stack>
  </Card>;
}

export function ApplicantHome() {
  const { session } = useAuth();
  const active = useLoad(loadActiveApplications, [], {});
  const closing = useLoad(loadClosingSoon, [], {});
  const name = firstName(session?.user.fullName);
  return <Stack spacing={4}>
    <PageHeader title={name ? `Welcome back, ${name}` : 'Welcome back'} description="Follow your applications and the jobs you saved."
      action={<Button component={Link} to="/applicant/jobs" variant="contained">Find jobs</Button>} />
    <ReadinessHint />
    <Section title="Active applications" seeAll={['/applicant/applications', 'All applications']}>
        {active.loading && <LoadingRows rows={2} height={120} label="Loading applications" />}
        {active.error && <ErrorState message={active.error} onRetry={active.refresh} />}
        {active.data && !active.data.length && <EmptyState title="No active applications"
          description="When you apply for a job, its progress shows here."
          action={<Button component={Link} to="/applicant/jobs" variant="outlined">Find jobs</Button>} />}
        {!!active.data?.length && <CardGrid>
          {active.data.slice(0, shown).map(x => <ApplicationCard key={x.app.applicationId} app={x.app} status={x.status} />)}
        </CardGrid>}
    </Section>
    <Section title="Saved jobs closing soon" seeAll={['/applicant/saved', 'All saved jobs']}>
        {closing.loading && <LoadingRows rows={2} label="Loading saved jobs" />}
        {closing.error && <ErrorState message={closing.error} onRetry={closing.refresh} />}
        {closing.data && !closing.data.length && <EmptyState description="None of your saved jobs closes in the next 7 days." />}
        {!!closing.data?.length && <CardGrid>
          {closing.data.slice(0, shown).map(job => <JobCard key={job.jobId} job={job} viewTo={`/applicant/jobs/${encodeURIComponent(job.jobId)}`} />)}
        </CardGrid>}
    </Section>
  </Stack>;
}
