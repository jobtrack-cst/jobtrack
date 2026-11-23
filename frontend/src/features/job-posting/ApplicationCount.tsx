import { Typography } from '@mui/material';
import { listApplications } from '../application/api';
import { useLoad } from '../application-status/useLoad';

// "3 applications" for one job, from ListApplications totalCount with the job_id filter.
export function ApplicationCount({ jobId }: { jobId: string }) {
  const count = useLoad(() => listApplications('', jobId), [jobId], {});
  if (count.loading) return <Typography variant="body2" color="text.secondary">Counting applications…</Typography>;
  if (count.error) return <Typography variant="body2" color="text.secondary">Applications could not be counted</Typography>;
  const n = Number(count.data?.pagination?.totalCount ?? 0);
  return <Typography variant="body2" color="text.secondary">{n === 1 ? '1 application' : `${n.toLocaleString()} applications`}</Typography>;
}
