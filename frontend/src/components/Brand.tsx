import { useState } from 'react';
import { Box, Typography } from '@mui/material';
import { Link } from 'react-router';

// Build-time asset: replace this local file, or change the path for a PNG.
const markAsset = '/branding/jobtrack-mark.svg';

export function Brand({ to }: { to: string }) {
  const [failed, setFailed] = useState(false);
  return <Box component={Link} to={to} aria-label="JobTrack home" sx={{ display: 'inline-flex', alignItems: 'center', gap: 1.25, minHeight: 44,
    textDecoration: 'none', color: 'text.primary', flexShrink: 0, borderRadius: 1,
    '&:hover img': { transform: 'translateY(-1px)' } }}>
    {failed ? <Box aria-hidden="true" sx={{ width: 38, height: 38, borderRadius: '10px', display: 'grid', placeItems: 'center',
      bgcolor: 'primary.dark', color: '#fff9ee', fontWeight: 800, fontSize: 16 }}>JT</Box>
      : <Box component="img" src={markAsset} alt="" width={38} height={38} onError={() => setFailed(true)}
        sx={{ objectFit: 'contain', display: 'block', transition: 'transform 160ms ease' }} />}
    <Typography component="span" sx={{ fontWeight: 800, fontSize: { xs: '1.2rem', sm: '1.35rem' }, letterSpacing: '-0.055em' }}>JobTrack<Box component="span" sx={{ color: 'secondary.main' }}>.</Box></Typography>
  </Box>;
}
