import { Link, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link as RouterLink } from 'react-router';

// A home page section: title, optional "See all" link, content.
export function Section({ title, seeAll, children }: { title: string; seeAll?: [to: string, label: string]; children: ReactNode }) {
  return <Stack component="section" spacing={2} sx={{ minWidth: 0 }} aria-label={title}>
    <Stack direction="row" spacing={2} sx={{ alignItems: 'baseline', justifyContent: 'space-between', flexWrap: 'wrap' }}>
      <Typography variant="h2">{title}</Typography>
      {seeAll && <Link component={RouterLink} to={seeAll[0]} sx={{ minHeight: 44, display: 'inline-flex', alignItems: 'center' }}>{seeAll[1]}</Link>}
    </Stack>
    {children}
  </Stack>;
}
