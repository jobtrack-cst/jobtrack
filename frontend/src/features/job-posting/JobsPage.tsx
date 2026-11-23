import { useState } from 'react';
import { Button, Stack } from '@mui/material';
import { Link } from 'react-router';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { JobCard } from '../../components/JobCard';
import { CardGrid } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager, pageSize } from '../../components/Pager';
import { StatusChip } from '../../components/StatusChip';
import { TabBar, TabPanel, useTab } from '../../components/Tabs';
import { rememberJob } from '../../components/useJob';
import { allPages } from '../application/applicant';
import { useLoad } from '../application-status/useLoad';
import { listJobs, type Job, type JobStatus } from './api';
import { ApplicationCount } from './ApplicationCount';
import { jobMessages } from './shared';

const tabs = ['active', 'drafts', 'closed'] as const;
type TabValue = typeof tabs[number];

const jobPath = (job: Job) => `/employer/jobs/${encodeURIComponent(job.jobId)}`;
const changedAt = (job: Job) => Date.parse(job.timestamps?.updatedAt ?? job.timestamps?.createdAt ?? '') || 0;

// Closed holds CLOSED and REMOVED jobs. ListMyJobs filters by one status, so both are
// read to the end and merged, most recently changed first; the Pager then pages here.
async function loadClosedJobs() {
  const read = (status: JobStatus) => allPages(token => listJobs(token, status).then(r => ({ items: r.jobs, pagination: r.pagination })));
  const [closed, removed] = await Promise.all([read('JOB_STATUS_CLOSED'), read('JOB_STATUS_REMOVED')]);
  const jobs = [...closed, ...removed].sort((a, b) => changedAt(b) - changedAt(a));
  jobs.forEach(rememberJob);
  return jobs;
}

function JobsGrid({ jobs, tab }: { jobs: Job[]; tab: TabValue }) {
  return <CardGrid label={tab === 'active' ? 'Active jobs' : tab === 'drafts' ? 'Draft jobs' : 'Closed jobs'}>
    {jobs.map(job => <JobCard key={job.jobId} job={job} viewTo={jobPath(job)}
      // Closed and removed jobs only open read-only, so they are never offered as Manage.
      viewLabel={tab === 'closed' ? 'View' : 'Manage'}
      chip={tab === 'closed' ? <StatusChip kind="job" status={job.status} /> : undefined}
      extra={tab === 'drafts' ? undefined : <ApplicationCount jobId={job.jobId} />} />)}
  </CardGrid>;
}

// Active and Drafts page on the server with the ListMyJobs status filter.
function ServerTab({ tab, status }: { tab: TabValue; status: JobStatus }) {
  const [token, setToken] = useState('');
  const list = useLoad(async () => {
    const page = await listJobs(token, status);
    page.jobs?.forEach(rememberJob);
    return page;
  }, [token, status], jobMessages);
  const jobs = list.data?.jobs ?? [];
  return <Stack spacing={2}>
    {list.loading && <LoadingRows rows={2} height={160} label="Loading jobs" />}
    {list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !jobs.length && (tab === 'active'
      ? <EmptyState title="No active jobs" description="Published jobs that accept applications show here."
        action={<Button component={Link} to="/employer/jobs/new" variant="outlined">Post a job</Button>} />
      : <EmptyState title="No drafts" description="A new job starts as a draft. Publish it when it is ready." />)}
    {!!jobs.length && <JobsGrid jobs={jobs} tab={tab} />}
    {list.data && <Pager token={token} nextToken={list.data.pagination?.nextPageToken} totalCount={list.data.pagination?.totalCount}
      count={jobs.length} busy={list.loading} onPage={setToken} />}
  </Stack>;
}

function ClosedTab() {
  const list = useLoad(loadClosedJobs, [], jobMessages);
  const [token, setToken] = useState('');
  const all = list.data ?? [];
  const offset = Number(token) || 0;
  const page = all.slice(offset, offset + pageSize);
  const next = offset + pageSize < all.length ? String(offset + pageSize) : undefined;
  return <Stack spacing={2}>
    {list.loading && <LoadingRows rows={2} height={160} label="Loading jobs" />}
    {list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !all.length && <EmptyState title="No closed jobs" description="Jobs you close, and jobs an admin removes, show here." />}
    {!!page.length && <JobsGrid jobs={page} tab="closed" />}
    {list.data && <Pager token={token} nextToken={next} totalCount={all.length} count={page.length} onPage={setToken} />}
  </Stack>;
}

// Jobs: Active, Drafts and Closed tabs (redesign doc Section 5, JT-111).
export function JobsPage() {
  const [tab, setTab] = useTab(tabs);
  return <Stack spacing={4}>
    <PageHeader title="Jobs" description="Your vacancies: open, in preparation and closed."
      action={<Button component={Link} to="/employer/jobs/new" variant="contained">Post a job</Button>} />
    <Stack spacing={3}>
      <TabBar id="jobs" label="Jobs" value={tab} onChange={setTab} tabs={[['active', 'Active'], ['drafts', 'Drafts'], ['closed', 'Closed']]} />
      <TabPanel id="jobs" value={tab}>
        {/* Keyed by tab, so each tab starts on its first page. */}
        {tab === 'active' && <ServerTab key="active" tab="active" status="JOB_STATUS_PUBLISHED" />}
        {tab === 'drafts' && <ServerTab key="drafts" tab="drafts" status="JOB_STATUS_DRAFT" />}
        {tab === 'closed' && <ClosedTab />}
      </TabPanel>
    </Stack>
  </Stack>;
}
