import { useState } from 'react';
import { Alert, Button, Chip, CircularProgress, Stack, Typography } from '@mui/material';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { getCompanyProfile, type CompanyProfile } from '../employer/api';
import { verificationOf } from '../employer/company';

const messages = {
  [grpcCode.notFound]: 'This employer has not created a company profile yet.',
  [grpcCode.invalidArgument]: 'The employer ID is not valid.',
};

// Loaded only when asked, so the request list does not make one call per row.
export function CompanyDetails({ employerId }: { employerId: string }) {
  const [state, setState] = useState<{ loading: boolean; profile?: CompanyProfile; error?: string }>();
  async function load() {
    setState({ loading: true });
    try { setState({ loading: false, profile: (await getCompanyProfile(employerId)).profile }); }
    catch (reason) { setState({ loading: false, error: gatewayMessage(reason, messages) }); }
  }
  if (!state) return <Button onClick={() => void load()}>View company</Button>;
  if (state.loading) return <CircularProgress aria-label="Loading company" size={20} />;
  if (state.error) return <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
    <Alert severity="warning">{state.error}</Alert><Button onClick={() => void load()}>Try again</Button>
  </Stack>;
  const p = state.profile;
  const view = verificationOf(p?.verificationStatus);
  return <Stack spacing={0.5} aria-label="Company details">
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      <Typography>Company: {p?.companyName || 'Not set'}</Typography><Chip size="small" label={view.label} color={view.color} />
    </Stack>
    {p?.website && <Typography>Website: {p.website}</Typography>}
    {p?.location && <Typography>Location: {p.location}</Typography>}
    {p?.description && <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{p.description}</Typography>}
  </Stack>;
}
