import { useEffect, useState } from 'react';
import { Button, Stack } from '@mui/material';
import { Link, useSearchParams } from 'react-router';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { CardGrid, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { MiniCard } from '../../components/MiniCard';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { SearchBar, SearchSelect } from '../../components/SearchBar';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { jobName, rememberJob, useJob } from '../../components/useJob';
import { ApplicantAvatar } from '../applicant-profile/ApplicantAvatar';
import { useLoad } from '../application-status/useLoad';
import { listApplications, type Application } from './api';
import { statusOf } from './applicant';
import { applicantLabel, loadFilterJobs } from './employer';
import { validId } from './shared';

function ApplicantCard({ app }: { app: Application }) {
  const job = useJob(app.jobId);
  const status = useLoad(() => statusOf(app), [app.applicationId, app.withdrawn], {});
  const name = applicantLabel(app);
  const title = jobName(job);
  const applied = shortDate(app.timestamps?.createdAt);
  return <MiniCard avatar={<ApplicantAvatar applicationId={app.applicationId} name={name} />} title={name}
    line={applied ? `${title}, applied ${applied}` : title}
    chip={status.data ? <StatusChip kind="application" status={status.data} /> : status.error ? <ToneChip label="Status unavailable" tone="neutral" /> : undefined}
    viewTo={`/employer/applicants/${encodeURIComponent(app.applicationId)}`} viewLabel={`View application from ${name} for ${title}`} />;
}

// Applicants (redesign doc Section 5, JT-111): a job filter (ListApplications job_id)
// over small cards with the applicant's photo or initials, name, job, applied date and
// current status. The chosen job is kept in ?job=, so Back and reloads keep it.
export function ApplicantsPage() {
  const [params, setParams] = useSearchParams();
  const raw = params.get('job') ?? '';
  const jobId = validId(raw) ? raw.toLowerCase() : '';
  const [choice, setChoice] = useState(jobId);
  // The page token belongs to the job it was read for, so a new job (from Show applicants,
  // Back or Forward) starts on page 1 in the same render that reads ?job=, and only one
  // request is sent for it.
  const [page, setPage] = useState({ jobId, token: '' });
  // Any change of ?job= clears the stored token, so returning to an earlier job (for
  // example All jobs) starts on page 1 instead of reusing its old page. Set during
  // render, so React renders again before any effect runs and no request uses the old token.
  if (page.jobId !== jobId) setPage({ jobId, token: '' });
  const token = page.jobId === jobId ? page.token : '';
  const setToken = (next: string) => setPage({ jobId, token: next });
  // Back and Forward change ?job=; the dropdown follows the address.
  useEffect(() => { setChoice(jobId); }, [jobId]);
  const jobs = useLoad(async () => {
    const options = await loadFilterJobs();
    options.forEach(o => rememberJob(o.job));
    return options;
  }, [], {});
  const list = useLoad(() => listApplications(token, jobId || undefined), [token, jobId], {});
  const options: [string, string][] = (jobs.data ?? []).map(o => [o.job.jobId.toLowerCase(), o.label]);
  // A job in the address that the list does not hold still filters; it is never shown by ID.
  if (choice && jobs.data && !options.some(([id]) => id === choice)) options.unshift([choice, 'Job no longer listed']);
  const chosen = options.find(([id]) => id === jobId)?.[1];
  function apply() {
    // The same job again only returns to page 1; a new job changes the address, which resets the page.
    if (choice === jobId) { setToken(''); return; }
    setParams(previous => {
      const next = new URLSearchParams(previous);
      if (choice) next.set('job', choice); else next.delete('job');
      return next;
    }, { replace: true });
  }
  const apps = list.data?.applications ?? [];
  const total = Number(list.data?.pagination?.totalCount ?? apps.length);
  return <Stack spacing={4}>
    <PageHeader title="Applicants" description="Everyone who applied for your jobs, with their current status." />
    <SearchBar label="Filter applicants" submitLabel="Show applicants" onSubmit={apply}>
      <SearchSelect label="Job" anyLabel="All jobs" value={choice} onChange={setChoice} options={options} disabled={jobs.loading} grow={3} />
    </SearchBar>
    {jobs.error && <ErrorState message="Your jobs could not be loaded for the filter. Applicants for all jobs are still shown." onRetry={jobs.refresh} />}
    <Stack spacing={2}>
      {list.data && !!apps.length && <ResultsHeader title={`${total.toLocaleString()} ${total === 1 ? 'applicant' : 'applicants'}`}
        note={jobId ? `For ${chosen ?? 'the chosen job'}` : 'All jobs'} />}
      {list.loading && <LoadingRows rows={3} height={140} label="Loading applicants" />}
      {list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
      {list.data && !apps.length && (jobId
        ? <EmptyState title="No applicants for this job" description="Applications for this job show here as they arrive." />
        : <EmptyState title="No applicants yet" description="Applications for your jobs show here as they arrive."
          action={<Button component={Link} to="/employer/jobs" variant="outlined">Your jobs</Button>} />)}
      {!!apps.length && <CardGrid label="Applicants">{apps.map(app => <ApplicantCard key={app.applicationId} app={app} />)}</CardGrid>}
      {list.data && <Pager token={token} nextToken={list.data.pagination?.nextPageToken} totalCount={list.data.pagination?.totalCount}
        count={apps.length} busy={list.loading} onPage={setToken} />}
    </Stack>
  </Stack>;
}
