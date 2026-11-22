import { useState } from 'react';
import { Alert, Box, Button, CircularProgress, Stack, Typography } from '@mui/material';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { getCompanyProfile, type CompanyProfile } from '../employer/api';
import { StatusChip } from '../../components/StatusChip';
import { tokens } from '../../theme';

const messages = {
  [grpcCode.notFound]: 'This employer has not created a company profile yet.',
  [grpcCode.invalidArgument]: 'The employer ID is not valid.',
};

// Loaded only when asked, so the request list does not make one profile call per row.
export function CompanyDetails({ employerId }: { employerId: string }) {
  const [state, setState] = useState<{ loading: boolean; profile?: CompanyProfile; error?: string }>();
  async function load() {
    setState({ loading: true });
    try { setState({ loading: false, profile: (await getCompanyProfile(employerId)).profile }); }
    catch (reason) { setState({ loading: false, error: gatewayMessage(reason, messages) }); }
  }
  if (!state) return <Box><Button variant="outlined" onClick={() => void load()}>Show company details</Button></Box>;
  if (state.loading) return <CircularProgress aria-label="Loading company" size={24} />;
  if (state.error) return <Alert severity="warning" action={<Button color="inherit" onClick={() => void load()}>Try again</Button>}>{state.error}</Alert>;
  const p = state.profile;
  return <Stack spacing={1} aria-label="Company details" sx={{ p: 2, borderRadius: '12px', bgcolor: tokens.surfaceRaised }}>
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
      <Typography variant="subtitle1">{p?.companyName || 'Company name not set'}</Typography>
      <StatusChip kind="verification" status={p?.verificationStatus} />
    </Stack>
    {p?.website && <Typography variant="body2">Website: {p.website}</Typography>}
    {p?.location && <Typography variant="body2">Location: {p.location}</Typography>}
    {p?.description && <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap' }}>{p.description}</Typography>}
  </Stack>;
}
