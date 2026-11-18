import { useState } from 'react';
import { Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { formatTime } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listJobs } from './api';
import { jobMessages, jobStatus } from './shared';

export function JobsPage() {
  const [token, setToken] = useState('');
  const resource = useLoad(() => listJobs(token), [token], jobMessages);
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">My jobs</Typography>
    <Button component={Link} to="/employer/jobs/new">Create job</Button>
    <ResourceState {...resource} />
    <Button disabled={resource.loading} onClick={resource.refresh}>Refresh jobs</Button>
    {resource.data && <Typography>Total jobs: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.jobs?.length && <Typography>No jobs on this page.</Typography>}
    {resource.data?.jobs?.map(job => <Card key={job.jobId}><CardContent><Stack spacing={1}>
      <Typography variant="h6">{job.title}</Typography><Typography>{jobStatus(job.status)}</Typography>
      <Typography>{job.location}</Typography><Typography>Deadline: {formatTime(job.deadline)}</Typography>
      <Button component={Link} to={`/employer/jobs/${job.jobId}`}>Manage job</Button>
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={resource.loading} onPage={setToken} />
  </Stack>;
}
