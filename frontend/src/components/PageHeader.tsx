import { Box, Link, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link as RouterLink } from 'react-router';
import { Icon } from './Icon';

// Every page starts with a title, one plain sentence and at most one primary action.
// Top-level pages use the 64px title; detail pages (with `back`) use 40px and a back link.
// `actions` is kept for pages not rebuilt yet.
export function PageHeader({ title, description, action, actions, back }: {
  title: string; description?: string; action?: ReactNode; actions?: ReactNode; back?: { to: string; label: string };
}) {
  const right = action ?? actions;
  const titleSx = back
    ? { fontSize: { xs: '2rem', sm: '2.5rem' }, lineHeight: 1.2, letterSpacing: '-0.04em' }
    // 64px from 900px up; smaller on phones so long titles never scroll sideways.
    : { fontSize: { xs: '2.5rem', sm: '3.25rem', md: '4rem' }, lineHeight: 1, letterSpacing: '-0.05em' };
  return <Stack spacing={1.5} sx={{ minWidth: 0 }}>
    {back && <Link component={RouterLink} to={back.to} sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, minHeight: 44, alignSelf: 'flex-start' }}>
      <Icon name="chevronLeft" size={18} />{back.label}
    </Link>}
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ xs: 'stretch', sm: 'flex-end' }}>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography variant="h1" sx={{ fontWeight: 800, color: 'text.primary', ...titleSx }}>{title}</Typography>
        {description && <Typography color="text.secondary" sx={{ mt: 1.5, maxWidth: '70ch', fontSize: '1.0625rem' }}>{description}</Typography>}
      </Box>
      {right && <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', flexShrink: 0, alignItems: 'center', maxWidth: '100%' }}>{right}</Stack>}
    </Stack>
  </Stack>;
}
