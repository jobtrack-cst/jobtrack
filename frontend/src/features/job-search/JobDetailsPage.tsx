import { Alert, Button, Stack, Typography } from '@mui/material';
import { Link, useParams } from 'react-router';
import { getJob } from '../application/api';
import { ResourceState, validId } from '../application/shared';
import { formatTime } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import type { Job } from '../job-posting/api';
import { jobStatus } from '../job-posting/shared';
import { SaveJobButton } from '../saved-jobs/SaveJobButton';

export function JobDetailsPage() {
  const { jobId = '' } = useParams();
  const resource = useLoad(async () => validId(jobId) ? (await getJob(jobId)).job as Job : undefined, [jobId], {});
  const job = resource.data;
  const open = job?.status === 'JOB_STATUS_PUBLISHED' && Date.parse(job.deadline) > Date.now();
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Job details</Typography><Button component={Link} to="/applicant/jobs">Search jobs</Button>
    {!validId(jobId) && <Alert severity="error">The job ID in the address is not valid.</Alert>}
    <ResourceState {...resource} /><Button disabled={resource.loading} onClick={resource.refresh}>Refresh job</Button>
    {job && <><Typography variant="h5" component="h2">{job.title}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{job.description}</Typography>
      <Typography>Location: {job.location || 'Not specified'}</Typography><Typography>Status: {jobStatus(job.status)}</Typography><Typography>Deadline: {formatTime(job.deadline)}</Typography>
      {open ? <><SaveJobButton key={jobId} jobId={jobId} /><Button component={Link} to={`/applicant/apply?jobId=${jobId}`}>Apply for this job</Button></>
        : <Alert severity="warning">This job is closed, unavailable for applications, or past its deadline.</Alert>}
      <Button component={Link} to={`/applicant/report-job/${jobId}`}>Report job</Button>
    </>}
  </Stack>;
}
