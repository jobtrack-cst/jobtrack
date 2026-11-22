import { Box, type BoxProps } from '@mui/material';
import { glassFlat } from '../theme';

// Glass-coloured panel without blur (see glassFlat in theme.ts).
export function GlassPanel({ sx, ...props }: BoxProps) {
  return <Box {...props} sx={[{ ...glassFlat, borderRadius: '20px', p: { xs: 2.5, sm: 3 }, minWidth: 0 }, ...(Array.isArray(sx) ? sx : [sx])]} />;
}
