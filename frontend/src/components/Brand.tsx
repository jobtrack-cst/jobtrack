import { Box, Typography } from '@mui/material';
import { Link } from 'react-router';

// The signed-out pages use the larger size (38px mark, 22px wordmark).
export function Brand({ to, onClick, large = false }: { to: string; onClick?: () => void; large?: boolean }) {
  // Top bar: 32px mark, 20px wordmark. Signed-out pages: 38px and 22px.
  const mark = large ? 38 : 32;
  return <Box component={Link} to={to} onClick={onClick} aria-label="JobTrack home" sx={{ display: 'inline-flex', alignItems: 'center', gap: 1.25, minHeight: 44,
    textDecoration: 'none', color: 'text.primary', flexShrink: 0, borderRadius: '12px', pr: 0.5 }}>
    {/* Decorative: the link's name already says JobTrack. */}
    <Box component="img" src="/branding/jobtrack-mark.svg" alt="" width={mark} height={mark} sx={{ display: 'block' }} />
    <Typography component="span" sx={{ fontWeight: 800, fontSize: large ? '1.375rem' : '1.25rem', letterSpacing: '-0.03em' }}>JobTrack</Typography>
  </Box>;
}
