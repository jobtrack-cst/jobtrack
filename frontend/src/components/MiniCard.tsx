import { Box, Button, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { tokens } from '../theme';

interface Props {
  // A circular avatar (company logo, photo or initials).
  avatar: ReactNode;
  title: string;
  // One line of supporting text, for example "Company, Location".
  line?: string;
  // One chip: a deadline or a status.
  chip?: ReactNode;
  viewTo: string;
  // Accessible name of the View button when "View <title>" is not unique enough.
  viewLabel?: string;
}

// Small minimal card for Saved jobs, My applications and Applicants (redesign doc
// Section 4): avatar, title, one line, one chip and one View button. Goes in a CardGrid.
export function MiniCard({ avatar, title, line, chip, viewTo, viewLabel }: Props) {
  return <Box component="li" sx={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2, p: 3, bgcolor: tokens.surface,
    border: `1px solid ${tokens.border}`, borderRadius: '20px', overflowWrap: 'anywhere' }}>
    <Stack direction="row" spacing={1.5} sx={{ alignItems: 'flex-start', justifyContent: 'space-between' }}>
      {avatar}
      {chip}
    </Stack>
    <Box sx={{ minWidth: 0 }}>
      <Typography component="h3" sx={{ fontSize: '1.1875rem', fontWeight: 700, lineHeight: 1.35 }}>{title}</Typography>
      {line && <Typography sx={{ fontSize: '0.9375rem', color: tokens.textSecondary, mt: 0.5 }}>{line}</Typography>}
    </Box>
    {/* The title names the card for screen readers, so the button says which one it opens. */}
    <Button component={Link} to={viewTo} variant="contained" aria-label={viewLabel ?? `View ${title}`}
      sx={{ mt: 'auto', minHeight: 44, borderRadius: '12px' }}>View</Button>
  </Box>;
}
