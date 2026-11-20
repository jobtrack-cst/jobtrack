import { Alert, Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { Link, useParams } from 'react-router';
import { getJob } from '../application/api';
import { ResourceState, validId } from '../application/shared';
import { formatTime } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import type { Job } from '../job-posting/api';
import { jobStatus } from '../job-posting/shared';
import { SaveJobButton } from '../saved-jobs/SaveJobButton';
import { JobCompany } from '../employer/JobCompany';

export function JobDetailsPage() {
  const { jobId = '' } = useParams();
  const resource = useLoad(async () => validId(jobId) ? (await getJob(jobId)).job as Job : undefined, [jobId], {});
  const job = resource.data;
  const open = job?.status === 'JOB_STATUS_PUBLISHED' && Date.parse(job.deadline) > Date.now();
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Job details</Typography><Button component={Link} to="/applicant/jobs">Search jobs</Button>
    {!validId(jobId) && <Alert severity="error">The job ID in the address is not valid.</Alert>}
    <ResourceState {...resource} /><Button disabled={resource.loading} onClick={resource.refresh}>Refresh job</Button>
    {job && <><Card sx={{ borderTop: '3px solid #c9a76b' }}><CardContent><Stack spacing={2}>
      <Typography variant="h5" component="h2">{job.title}</Typography><JobCompany employerId={job.employerId} size={48} /><Typography sx={{ whiteSpace: 'pre-wrap' }}>{job.description}</Typography>
      <Stack spacing={0.5} sx={{ p: 2, bgcolor: '#f1f4eb', borderRadius: '10px' }}>
        <Typography>Location: {job.location || 'Not specified'}</Typography><Typography>Status: {jobStatus(job.status)}</Typography><Typography>Deadline: {formatTime(job.deadline)}</Typography>
      </Stack>
    </Stack></CardContent></Card>
      {open ? <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}><Button variant="contained" component={Link} to={`/applicant/apply?jobId=${jobId}`}>Apply for this job</Button><SaveJobButton key={jobId} jobId={jobId} /></Stack>
        : <Alert severity="warning">This job is closed, unavailable for applications, or past its deadline.</Alert>}
      <Button component={Link} to={`/applicant/report-job/${jobId}`}>Report job</Button>
    </>}
  </Stack>;
}
