import { Box, Skeleton, Stack } from '@mui/material';
import { tokens } from '../theme';

// Skeleton rows shaped like the list that is loading. Screen readers hear the label once.
export function LoadingRows({ rows = 3, label = 'Loading', height = 72 }: { rows?: number; label?: string; height?: number }) {
  return <Stack spacing={1.5} role="status" aria-label={label} aria-live="polite">
    {Array.from({ length: rows }, (_, i) => <Box key={i} aria-hidden="true" sx={{ p: 2, borderRadius: '20px', border: `1px solid ${tokens.border}`, bgcolor: tokens.surface }}>
      <Skeleton variant="text" width="45%" sx={{ fontSize: '1.125rem' }} />
      <Skeleton variant="text" width="70%" />
      {height > 72 && <Skeleton variant="rounded" height={height - 72} sx={{ mt: 1 }} />}
    </Box>)}
  </Stack>;
}
