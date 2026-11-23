import type { ReactNode } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { DeadlineTag } from '../../components/DeadlineTag';
import { TagChip } from '../../components/JobCard';
import { StatusChip } from '../../components/StatusChip';
import { formatTime } from '../application-status/shared';
import type { Category, Tag } from '../job-alert/api';
import type { Job } from './api';
import { ApplicationCount } from './ApplicationCount';
import { categoryName, tagName } from './shared';

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return <Box sx={{ minWidth: 0 }}>
    <Typography component="h3" sx={{ fontSize: '0.875rem', fontWeight: 600, color: 'text.secondary', mb: 0.5 }}>{label}</Typography>
    {children}
  </Box>;
}

// Read-only job details for the owner: every field of the job, with category and tag
// names from category-tag (never their IDs), and the application count once published.
export function JobDetails({ job, categories, tags }: { job: Job; categories: Category[]; tags: Tag[] }) {
  const tagIds = job.tagIds ?? [];
  return <Stack spacing={2.5}>
    <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', rowGap: 1 }}>
      <StatusChip kind="job" status={job.status} />
      {job.status === 'JOB_STATUS_PUBLISHED' && <DeadlineTag deadline={job.deadline} />}
    </Stack>
    <Box sx={{ display: 'grid', gap: 2.5, gridTemplateColumns: { xs: 'minmax(0, 1fr)', sm: 'repeat(2, minmax(0, 1fr))' } }}>
      <Detail label="Deadline"><Typography>{formatTime(job.deadline)}</Typography></Detail>
      <Detail label="Location"><Typography color={job.location ? 'text.primary' : 'text.secondary'}>{job.location || 'No location.'}</Typography></Detail>
      <Detail label="Category">
        <Typography color={job.categoryId ? 'text.primary' : 'text.secondary'}>{categoryName(categories, job.categoryId)}</Typography>
      </Detail>
      {job.status !== 'JOB_STATUS_DRAFT' && <Detail label="Applications"><ApplicationCount jobId={job.jobId} /></Detail>}
    </Box>
    <Detail label="Tags">
      {tagIds.length
        ? <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>{tagIds.map(id => <TagChip key={id} label={tagName(tags, id)} />)}</Stack>
        : <Typography color="text.secondary">No tags.</Typography>}
    </Detail>
    <Detail label="Description">
      <Typography color={job.description ? 'text.primary' : 'text.secondary'} sx={{ whiteSpace: 'pre-line' }}>{job.description || 'No description.'}</Typography>
    </Detail>
  </Stack>;
}
