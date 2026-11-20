import { Box, Typography, SvgIcon } from '@mui/material';
import type { ReactNode } from 'react';

export function EmptyState({ children }: { children: ReactNode }) {
  return <Box role="status" sx={{ p: { xs: 2.5, sm: 3 }, border: '1px dashed', borderColor: '#c6d3c2', borderRadius: '14px',
    bgcolor: '#f0f3e9', display: 'flex', alignItems: 'center', gap: 2 }}>
    <Box sx={{ width: 42, height: 42, flexShrink: 0, display: 'grid', placeItems: 'center', borderRadius: '12px', bgcolor: '#e4ead9', color: 'primary.main' }}>
      <SvgIcon aria-hidden="true" sx={{ fontSize: 23 }}><path d="M4 5h16v14H4zM4 13h5l2 3h2l2-3h5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" /></SvgIcon>
    </Box>
    <Typography variant="body2" color="text.secondary">{children}</Typography>
  </Box>;
}
