import { Box, Stack, Typography, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { tokens } from '../theme';

// "4 open roles in Software" with "Soonest deadline first" on the right.
export function ResultsHeader({ title, note }: { title: string; note?: string }) {
  return <Stack direction="row" spacing={2} sx={{ alignItems: 'baseline', justifyContent: 'space-between', flexWrap: 'wrap' }}>
    <Typography component="h2" sx={{ fontSize: '1.5rem', fontWeight: 800, letterSpacing: '-0.02em' }}>{title}</Typography>
    {note && <Typography variant="body2" color="text.secondary">{note}</Typography>}
  </Stack>;
}

// Browse pages: a grid of cards at least 280px wide (the full width on small phones).
// auto-fill keeps empty tracks, so one card keeps its size instead of filling the row.
export function CardGrid({ children, label }: { children: ReactNode; label?: string }) {
  return <Box component="ul" aria-label={label} sx={{ listStyle: 'none', m: 0, p: 0, display: 'grid', gap: 2,
    gridTemplateColumns: 'repeat(auto-fill, minmax(min(280px, 100%), 1fr))' }}>{children}</Box>;
}

// Dense lists: rows inside one panel with dividers.
export function ListPanel({ children, label }: { children: ReactNode; label?: string }) {
  return <Box component="ul" aria-label={label} sx={{ listStyle: 'none', m: 0, p: 0, bgcolor: tokens.surface, border: `1px solid ${tokens.border}`,
    borderRadius: '20px', overflow: 'hidden', '& > li + li': { borderTop: `1px solid ${tokens.border}` } }}>{children}</Box>;
}

const rowSx = { display: 'block', px: { xs: 2, sm: 3 }, py: 2.5, minWidth: 0, overflowWrap: 'anywhere', color: 'text.primary', textDecoration: 'none' } as const;

// One row; with `to` the whole row is one link, so put no other controls in it then.
export function ListRow({ children, to, sx }: { children: ReactNode; to?: string; sx?: SxProps<Theme> }) {
  const extra = Array.isArray(sx) ? sx : [sx];
  return <Box component="li" sx={{ minWidth: 0 }}>
    {to
      ? <Box component={Link} to={to} sx={[rowSx, { transition: 'background-color 180ms ease', '&:hover': { bgcolor: tokens.surfaceRaised },
        '&:focus-visible': { outline: `3px solid ${tokens.accent}`, outlineOffset: -3 } }, ...extra]}>{children}</Box>
      : <Box sx={[rowSx, ...extra]}>{children}</Box>}
  </Box>;
}

// Forms sit in one panel with 32px padding.
export function FormPanel({ children, sx }: { children: ReactNode; sx?: SxProps<Theme> }) {
  return <Box sx={[{ bgcolor: tokens.surface, border: `1px solid ${tokens.border}`, borderRadius: '20px', p: { xs: 2.5, sm: 4 }, minWidth: 0 },
    ...(Array.isArray(sx) ? sx : [sx])]}>{children}</Box>;
}
