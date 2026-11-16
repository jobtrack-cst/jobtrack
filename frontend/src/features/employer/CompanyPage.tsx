import { useEffect, useState } from 'react';
import { Alert, Button, Chip, Stack, TextField, Typography } from '@mui/material';
import { Link } from 'react-router';
import { ResourceState } from '../application/shared';
import { formatTime, gatewayMessage, grpcCode, isNotFound, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { createCompanyProfile, getMyCompanyProfile, updateCompanyProfile, type CompanyFields, type CompanyProfile } from './api';
import { companyProblems, emptyCompany, fromProfile, trimCompany, verificationOf } from './company';

const messages = {
  [grpcCode.invalidArgument]: 'The profile was not saved. Company name needs 1 to 150 characters, description at most 2000, website a full http or https address of at most 255, and location at most 100.',
  [grpcCode.alreadyExists]: 'You already have a company profile. Refresh the page to edit it.',
  [grpcCode.permissionDenied]: 'Only employers can manage a company profile.',
  [grpcCode.notFound]: 'You do not have a company profile yet. Refresh the page to create one.',
};

export function CompanyPage() {
  const existing = useLoad(getMyCompanyProfile, [], messages);
  const [profile, setProfile] = useState<CompanyProfile>();
  const [form, setForm] = useState<CompanyFields>(emptyCompany);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  useEffect(() => {
    if (existing.data) { setProfile(existing.data.profile); setForm(fromProfile(existing.data.profile)); }
  }, [existing.data]);
  const creating = !profile && !existing.loading && isNotFound(existing.failure);
  const problems = companyProblems(form);
  const ready = length(form.companyName) > 0 && problems.length === 0;
  const status = verificationOf(profile?.verificationStatus);
  const verified = profile?.verificationStatus === 'VERIFICATION_STATUS_VERIFIED';
  const rejected = profile?.verificationStatus === 'VERIFICATION_STATUS_REJECTED';
  function update(key: keyof CompanyFields, value: string) { setForm(f => ({ ...f, [key]: value })); setNotice(''); }
  async function save() {
    if (busy || !ready) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const fields = trimCompany(form);
      const result = profile ? await updateCompanyProfile(fields) : await createCompanyProfile(fields);
      setNotice(profile ? 'Company profile saved.' : 'Company profile created. Next, request verification so you can publish jobs.');
      setProfile(result.profile); setForm(fromProfile(result.profile));
    } catch (reason) { setError(gatewayMessage(reason, messages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Company profile</Typography>
    {existing.loading && !profile && <ResourceState loading error="" />}
    {!existing.loading && existing.error && !creating && !profile && <>
      <Alert severity="error">{existing.error}</Alert><Button onClick={existing.refresh}>Try again</Button>
    </>}
    {profile && <Stack spacing={1}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Typography>Verification:</Typography><Chip label={status.label} color={status.color} />
      </Stack>
      {rejected && <Alert severity="warning">Verification was rejected. Reason: {profile.rejectionReason || 'Not given'}. You can send a new request.</Alert>}
      {!verified && <Button component={Link} to="/employer/verification">Request verification</Button>}
      <Typography variant="body2">Last updated: {formatTime(profile.timestamps?.updatedAt)}</Typography>
    </Stack>}
    {creating && <Typography>You do not have a company profile yet. Create one before you request verification.</Typography>}
    {(profile || creating) && <Stack component="form" spacing={2} noValidate onSubmit={e => { e.preventDefault(); void save(); }}>
      <TextField label="Company name" required value={form.companyName} disabled={busy} helperText="1 to 150 characters"
        onChange={e => update('companyName', e.target.value)} />
      <TextField label="Description" multiline minRows={3} value={form.description} disabled={busy} helperText="Optional, up to 2000 characters"
        onChange={e => update('description', e.target.value)} />
      <TextField label="Website" value={form.website} disabled={busy} placeholder="https://example.bt" helperText="Optional, a full http or https address"
        onChange={e => update('website', e.target.value)} />
      <TextField label="Location" value={form.location} disabled={busy} helperText="Optional, up to 100 characters"
        onChange={e => update('location', e.target.value)} />
      {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
      {error && <Alert severity="error">{error}</Alert>}
      {notice && <Alert severity="success">{notice}</Alert>}
      <Button type="submit" variant="contained" disabled={busy || !ready}>{profile ? 'Save profile' : 'Create profile'}</Button>
    </Stack>}
  </Stack>;
}
