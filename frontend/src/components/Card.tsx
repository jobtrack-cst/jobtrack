import { Box, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { tokens } from '../theme';

const base = {
  display: 'block', minWidth: 0, p: { xs: 2.5, sm: 3 }, borderRadius: '20px', border: `1px solid ${tokens.border}`,
  bgcolor: tokens.surface, color: 'text.primary', overflowWrap: 'anywhere', textDecoration: 'none',
} as const;

// A bordered surface with no shadow. With `to` the whole card is one link;
// put no other links or buttons inside it then.
export function Card({ children, to, label, sx, component = 'div' }: {
  children: ReactNode; to?: string; label?: string; sx?: SxProps<Theme>; component?: 'div' | 'article' | 'section' | 'li';
}) {
  const extra = Array.isArray(sx) ? sx : [sx];
  if (to) {
    const link = <Box component={Link} to={to} aria-label={label} sx={[base, {
      transition: 'background-color 180ms ease, border-color 180ms ease',
      '&:hover': { bgcolor: tokens.surfaceRaised, borderColor: tokens.textMuted },
      '&:focus-visible': { outline: `3px solid ${tokens.accent}`, outlineOffset: 2 },
    }, ...extra]}>{children}</Box>;
    // A list item keeps the list valid; the link fills it.
    return component === 'li' ? <Box component="li" sx={{ minWidth: 0 }}>{link}</Box> : link;
  }
  // Without a link, label names the region (for example a section).
  return <Box component={component} aria-label={label} sx={[base, ...extra]}>{children}</Box>;
}
