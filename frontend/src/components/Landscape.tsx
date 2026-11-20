import { Box } from '@mui/material';

// Original local vector artwork; decorative so it never repeats nearby copy.
export function Landscape() {
  return <Box component="img" src="/branding/valley.svg" alt="" aria-hidden="true" width={900} height={480}
    sx={{ display: 'block', width: '100%', height: '100%', objectFit: 'cover', objectPosition: '50% 60%', pointerEvents: 'none', borderRadius: 'inherit' }} />;
}
