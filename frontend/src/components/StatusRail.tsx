import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '../theme';
import { StatusChip } from './StatusChip';

// Read by screen readers, not shown.
const visuallyHidden = { position: 'absolute', width: 1, height: 1, p: 0, m: '-1px', overflow: 'hidden', clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', border: 0 } as const;

const steps = ['Submitted', 'Under review', 'Shortlisted', 'Interview', 'Offer'] as const;
const stepOf: Record<string, number> = {
  APPLICATION_STATUS_SUBMITTED: 0, APPLICATION_STATUS_UNDER_REVIEW: 1, APPLICATION_STATUS_SHORTLISTED: 2,
  APPLICATION_STATUS_INTERVIEW: 3, APPLICATION_STATUS_OFFERED: 4,
  // Accepted and declined always follow an offer.
  APPLICATION_STATUS_ACCEPTED: 4, APPLICATION_STATUS_DECLINED: 4,
};
const finalStates = new Set(['APPLICATION_STATUS_ACCEPTED', 'APPLICATION_STATUS_DECLINED', 'APPLICATION_STATUS_REJECTED', 'APPLICATION_STATUS_WITHDRAWN']);
export const isFinalStatus = (status?: string) => finalStates.has(status ?? '');

interface Props {
  status?: string;
  // For Rejected and Withdrawn: the last step reached before it, when known (from the history).
  reachedStatus?: string;
}

// The five-step progress bar with the current step highlighted, plus a chip for a final state.
export function StatusRail({ status, reachedStatus }: Props) {
  const final = isFinalStatus(status);
  const current = stepOf[status ?? ''] ?? (final ? stepOf[reachedStatus ?? ''] ?? -1 : -1);
  const stopped = status === 'APPLICATION_STATUS_REJECTED' || status === 'APPLICATION_STATUS_WITHDRAWN';
  const summary = current < 0 ? 'Status not available yet'
    : `Step ${current + 1} of 5, ${steps[current]}${final ? '' : ', current'}`;
  return <Stack spacing={1.25} sx={{ minWidth: 0 }}>
    <Box component="ol" aria-label={`Application progress: ${summary}`} sx={{ listStyle: 'none', m: 0, p: 0, display: 'grid', gridTemplateColumns: 'repeat(5, minmax(0, 1fr))', gap: 0.75 }}>
      {steps.map((label, i) => {
        const done = i < current || (i === current && final && !stopped);
        const active = i === current && !final;
        const color = active ? tokens.accent : done ? tokens.success : i === current && stopped ? tokens.textMuted : tokens.border;
        return <Box component="li" key={label} aria-current={active ? 'step' : undefined} sx={{ minWidth: 0 }}>
          <Box aria-hidden="true" sx={{ height: 6, borderRadius: 3, bgcolor: color, opacity: stopped && i > current ? 0.5 : 1 }} />
          {/* Step names are read out; on screen one caption names the current step, so narrow cards never break words. */}
          <Box component="span" sx={visuallyHidden}>{label}</Box>
        </Box>;
      })}
    </Box>
    {!final && <Typography variant="body2" aria-hidden="true" sx={{ color: 'text.secondary' }}>
      {current < 0 ? 'Status not available yet' : <><Box component="strong" sx={{ color: 'text.primary' }}>{steps[current]}</Box>, step {current + 1} of 5</>}
    </Typography>}
    {final && <Box><StatusChip kind="application" status={status} /></Box>}
  </Stack>;
}
