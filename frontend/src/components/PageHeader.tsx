import { Box, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export function PageHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ xs: 'stretch', md: 'flex-start' }}>
    <Box sx={{ minWidth: 0, flex: 1 }}>
      <Typography variant="h4" component="h1">{title}</Typography>
      {description && <Typography color="text.secondary" sx={{ mt: 1, maxWidth: '75ch' }}>{description}</Typography>}
    </Box>
    {actions && <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', flexShrink: 0, alignItems: 'center', maxWidth: '100%' }}>{actions}</Stack>}
  </Stack>;
}
