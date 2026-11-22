import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import type { Job } from '../features/job-posting/api';
import { tokens } from '../theme';
import { LogoSquare, useCompany } from './CompanyAvatar';
import { DeadlineTag } from './DeadlineTag';

// Tag chips: 13px, 600, radius 8 (status pills stay fully round).
export function TagChip({ label }: { label: string }) {
  return <Chip label={label} sx={{ borderRadius: '8px', fontSize: '0.8125rem', fontWeight: 600, bgcolor: tokens.chipBg, color: tokens.chipText }} />;
}

interface Props {
  job: Pick<Job, 'jobId' | 'title' | 'deadline'> & Partial<Pick<Job, 'employerId' | 'location'>>;
  viewTo: string;
  viewLabel?: string;
  // Tag names, never IDs.
  tags?: string[];
  // One line under the company, for example an application count.
  extra?: ReactNode;
  // Next to View job, for example Save.
  secondary?: ReactNode;
}

// Job card for browse pages (redesign doc Section 4).
export function JobCard({ job, viewTo, viewLabel = 'View job', tags = [], extra, secondary }: Props) {
  const { company } = useCompany(job.employerId);
  const meta = [company?.name, job.location].filter(Boolean).join(', ');
  return <Box component="li" sx={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2, p: 3, bgcolor: tokens.surface,
    border: `1px solid ${tokens.border}`, borderRadius: '20px', overflowWrap: 'anywhere' }}>
    <Stack direction="row" spacing={1.5} sx={{ alignItems: 'flex-start', justifyContent: 'space-between' }}>
      <LogoSquare src={company?.url} name={company?.name} size={48} />
      <DeadlineTag deadline={job.deadline} />
    </Stack>
    <Box sx={{ minWidth: 0 }}>
      <Typography component="h3" sx={{ fontSize: '1.1875rem', fontWeight: 700, lineHeight: 1.35 }}>{job.title || 'Untitled job'}</Typography>
      {meta && <Typography sx={{ fontSize: '0.9375rem', color: tokens.textSecondary, mt: 0.5 }}>{meta}</Typography>}
      {extra && <Box sx={{ mt: 0.5 }}>{extra}</Box>}
    </Box>
    {!!tags.length && <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1 }}>{tags.map(t => <TagChip key={t} label={t} />)}</Stack>}
    <Stack direction="row" spacing={1} sx={{ mt: 'auto' }}>
      <Button component={Link} to={viewTo} variant="contained" sx={{ flex: 1, minHeight: 44, borderRadius: '12px' }}>{viewLabel}</Button>
      {secondary}
    </Stack>
  </Box>;
}
