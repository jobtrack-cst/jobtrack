import { useEffect, useState } from 'react';
import { Button, Stack } from '@mui/material';
import { Link } from 'react-router';
import { LogoSquare, useCompany } from '../../components/CompanyAvatar';
import { DeadlineTag } from '../../components/DeadlineTag';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { CardGrid, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { MiniCard } from '../../components/MiniCard';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { jobName, useJob } from '../../components/useJob';
import { useLoad } from '../application-status/useLoad';
import { listSavedJobs, savedMessages, type SavedJob } from './api';
import { forgetSavedIds } from './savedState';

function SavedCard({ saved }: { saved: SavedJob }) {
  const state = useJob(saved.jobId);
  const { company } = useCompany(state.job?.employerId);
  const line = [company?.name, state.job?.location].filter(Boolean).join(', ');
  return <MiniCard avatar={<LogoSquare src={company?.url} name={company?.name} size={48} />} title={jobName(state)} line={line || undefined}
    chip={state.job && <DeadlineTag deadline={state.job.deadline} />} viewTo={`/applicant/jobs/${encodeURIComponent(saved.jobId)}`} />;
}

// Saved jobs as small minimal cards; Unsave is on each job's page.
export function SavedJobsPage() {
  const [token, setToken] = useState('');
  const resource = useLoad(() => listSavedJobs(token), [token], savedMessages);
  // This page reads the server; Save buttons elsewhere start from a fresh list afterwards.
  useEffect(() => forgetSavedIds(), []);
  const saved = resource.data?.savedJobs ?? [];
  const total = Number(resource.data?.pagination?.totalCount ?? saved.length);
  return <Stack spacing={4}>
    <PageHeader title="Saved jobs" description="Jobs you bookmarked. A job leaves this list shortly after it closes."
      action={<Button component={Link} to="/applicant/jobs" variant="contained">Find jobs</Button>} />
    <Stack component="section" aria-label="Saved jobs" spacing={2}>
      {resource.data && !!saved.length && <ResultsHeader title={`${total.toLocaleString()} saved ${total === 1 ? 'job' : 'jobs'}`} note="Newest first" />}
      {resource.loading && <LoadingRows rows={3} height={140} label="Loading saved jobs" />}
      {resource.error && <ErrorState message={resource.error} onRetry={resource.refresh} />}
      {resource.data && !saved.length && <EmptyState title="No saved jobs" description="Save a job from Find jobs to keep it here."
        action={<Button component={Link} to="/applicant/jobs" variant="outlined">Find jobs</Button>} />}
      {!!saved.length && <CardGrid label="Saved jobs">{saved.map(s => <SavedCard key={s.jobId} saved={s} />)}</CardGrid>}
      {resource.data && <Pager token={token} nextToken={resource.data.pagination?.nextPageToken} totalCount={resource.data.pagination?.totalCount}
        count={saved.length} busy={resource.loading} onPage={setToken} />}
    </Stack>
  </Stack>;
}
