import { EmptyState } from '../../components/EmptyState';
import { useState } from 'react';
import { Box, Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { formatTime } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listJobs } from './api';
import { jobMessages, jobStatus } from './shared';
import { PageHeader } from '../../components/PageHeader';

export function JobsPage() {
  const [token, setToken] = useState('');
  const resource = useLoad(() => listJobs(token), [token], jobMessages);
  return <Stack spacing={2}>
    <PageHeader title="My jobs" description="Create opportunities. Keep each vacancy and its publishing status in view." actions={<>
      <Button disabled={resource.loading} onClick={resource.refresh}>Refresh jobs</Button>
      <Button component={Link} variant="contained" to="/employer/jobs/new">Create job</Button>
    </>} />
    <ResourceState {...resource} />
    {resource.data && <Typography variant="subtitle2" color="text.secondary">Total jobs: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.jobs?.length && <EmptyState>No jobs on this page.</EmptyState>}
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'repeat(2, minmax(0, 1fr))' }, gap: 2 }}>
      {resource.data?.jobs?.map(job => <Card key={job.jobId}><CardContent><Stack spacing={1.5}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
          <Typography variant="overline" color="text.secondary">Vacancy</Typography><Chip label={jobStatus(job.status)} color={job.status === 'JOB_STATUS_PUBLISHED' ? 'success' : 'default'} variant="outlined" />
        </Stack>
        <Typography variant="h6" component="h2">{job.title}</Typography><Typography variant="body2" color="text.secondary">{job.location}</Typography>
        <Typography variant="body2" color="text.secondary">Deadline: {formatTime(job.deadline)}</Typography>
        <Stack className="record-actions"><Button variant="outlined" component={Link} to={`/employer/jobs/${job.jobId}`}>Manage job</Button></Stack>
      </Stack></CardContent></Card>)}
    </Box>
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={resource.loading} onPage={setToken} />
  </Stack>;
}
