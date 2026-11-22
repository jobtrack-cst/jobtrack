import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { tokens } from '../theme';

// A headline, one line and at most one action. Pages not rebuilt yet pass
// their single sentence as children.
export function EmptyState({ title, description, action, children }: { title?: string; description?: ReactNode; action?: ReactNode; children?: ReactNode }) {
  const line = description ?? children;
  return <Box role="status" sx={{ p: { xs: 2.5, sm: 3 }, border: `1px dashed ${tokens.border}`, borderRadius: '20px', bgcolor: 'rgba(30,38,64,0.6)',
    display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 1 }}>
    {title && <Typography variant="h3" component="p">{title}</Typography>}
    {line && <Typography color="text.secondary" variant={title ? 'body1' : 'body2'}>{line}</Typography>}
    {action && <Box sx={{ mt: 1 }}>{action}</Box>}
  </Box>;
}
