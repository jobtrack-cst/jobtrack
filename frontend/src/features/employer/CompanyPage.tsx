import { useEffect, useState, type ReactNode } from 'react';
import { Alert, Box, Button, Link, Stack, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { FormPanel } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { TextInput } from '../../components/TextInput';
import { gatewayMessage, grpcCode, isNotFound, length } from '../application-status/shared';
import { VerificationSection } from '../company-verification/VerificationSection';
import { createCompanyProfile, getMyCompanyProfile, updateCompanyProfile, type CompanyFields, type CompanyProfile } from './api';
import { companyProblems, emptyCompany, fromProfile, sameCompany, trimCompany, webAddress } from './company';
import { CompanyLogoUpload } from './CompanyLogoUpload';

const messages = {
  [grpcCode.invalidArgument]: 'The profile was not saved. Company name needs 1 to 150 characters, description at most 2000, website a full http or https address of at most 255, and location at most 100.',
  [grpcCode.alreadyExists]: 'You already have a company profile. Refresh the page to edit it.',
  [grpcCode.permissionDenied]: 'Only employers can manage a company profile.',
  [grpcCode.notFound]: 'You do not have a company profile yet. Refresh the page to create one.',
};

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return <Box sx={{ minWidth: 0 }}>
    <Typography component="h3" sx={{ fontSize: '0.875rem', fontWeight: 600, color: 'text.secondary', mb: 0.5 }}>{label}</Typography>
    {children}
  </Box>;
}

// Read-only view of the saved profile; Edit switches to the form (JT-109).
function CompanyView({ profile }: { profile: CompanyProfile }) {
  const website = profile.website?.trim() ?? '';
  return <Stack spacing={2.5}>
    <Detail label="Company name"><Typography sx={{ fontWeight: 700 }}>{profile.companyName || 'Not set'}</Typography></Detail>
    <Detail label="Description">
      <Typography color={profile.description ? 'text.primary' : 'text.secondary'} sx={{ whiteSpace: 'pre-line' }}>{profile.description || 'No description.'}</Typography>
    </Detail>
    <Detail label="Website">
      {website && webAddress(website)
        ? <Link href={website} target="_blank" rel="noopener noreferrer">{website}</Link>
        : <Typography color="text.secondary">{website || 'No website.'}</Typography>}
    </Detail>
    <Detail label="Location"><Typography color={profile.location ? 'text.primary' : 'text.secondary'}>{profile.location || 'No location.'}</Typography></Detail>
  </Stack>;
}

// Company: the profile (read-only with Edit, JT-109), the logo (JT-102) and verification
// (JT-33), all on one page. Without a profile the page opens on the create form.
export function CompanyPage() {
  const [profile, setProfile] = useState<CompanyProfile | null>();
  const [loadError, setLoadError] = useState('');
  const [revision, setRevision] = useState(0);
  const [form, setForm] = useState<CompanyFields>(emptyCompany);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // A reload keeps the shown profile (and an open edit) until the new one arrives.
  useEffect(() => {
    let active = true;
    setLoadError('');
    getMyCompanyProfile().then(({ profile: next }) => {
      if (!active) return;
      // The form is filled from the profile when Edit is pressed, so an open edit is kept.
      setProfile(next);
    }, reason => {
      if (!active) return;
      if (isNotFound(reason)) { setProfile(null); setForm(emptyCompany); }
      else setLoadError(gatewayMessage(reason, messages));
    });
    return () => { active = false; };
  }, [revision]);
  const problems = companyProblems(form);
  const changed = !profile || !sameCompany(form, fromProfile(profile));
  const ready = length(form.companyName) > 0 && problems.length === 0 && changed;
  function update(key: keyof CompanyFields, value: string) { setForm(f => ({ ...f, [key]: value })); setNotice(''); }
  function edit() { if (profile) setForm(fromProfile(profile)); setNotice(''); setError(''); setEditing(true); }
  function cancel() { if (profile) setForm(fromProfile(profile)); setError(''); setEditing(false); }
  async function save() {
    if (busy || !ready) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const fields = trimCompany(form);
      const result = profile ? await updateCompanyProfile(fields) : await createCompanyProfile(fields);
      setNotice(profile ? 'Company profile saved.' : 'Company profile created. Next, request verification so you can publish jobs.');
      setProfile(result.profile); setForm(fromProfile(result.profile)); setEditing(false);
    } catch (reason) { setError(gatewayMessage(reason, messages)); }
    finally { setBusy(false); }
  }
  const header = <PageHeader title="Company" description="Your company profile, logo and verification. Applicants see the profile on your jobs." />;
  if (profile === undefined) return <Stack spacing={4}>{header}
    {loadError ? <ErrorState message={loadError} onRetry={() => setRevision(n => n + 1)} /> : <LoadingRows rows={2} height={160} label="Loading company profile" />}
  </Stack>;
  const updated = shortDate(profile?.timestamps?.updatedAt);
  return <Stack spacing={4}>
    {header}
    {loadError && <ErrorState message={loadError} onRetry={() => setRevision(n => n + 1)} />}
    {notice && <Alert severity="success" role="status">{notice}</Alert>}
    {!profile && <Alert severity="info">You do not have a company profile yet. Create one before you request verification.</Alert>}
    {profile && !editing && <FormPanel>
      <Stack spacing={3} component="section" aria-label="Company profile">
        <Stack direction="row" spacing={2} sx={{ alignItems: 'baseline', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 0.5 }}>
          <Typography variant="h2" component="h2">Company profile</Typography>
          {updated && <Typography variant="body2" color="text.secondary">Last updated {updated}</Typography>}
        </Stack>
        <CompanyView profile={profile} />
        <Button variant="contained" onClick={edit} sx={{ alignSelf: 'flex-start' }}>Edit profile</Button>
      </Stack>
    </FormPanel>}
    {(!profile || editing) && <FormPanel>
      <Stack component="form" spacing={2.5} noValidate aria-label="Company profile" onSubmit={e => { e.preventDefault(); void save(); }}>
        <Typography variant="h2" component="h2">{profile ? 'Edit company profile' : 'Create company profile'}</Typography>
        <TextInput label="Company name" required value={form.companyName} disabled={busy}
          helperText={`${length(form.companyName)} of 150 characters`} onChange={e => update('companyName', e.target.value)} />
        <TextInput label="Description" multiline minRows={3} value={form.description} disabled={busy}
          helperText={`Optional. ${length(form.description)} of 2000 characters.`} onChange={e => update('description', e.target.value)} />
        <TextInput label="Website" value={form.website} disabled={busy} placeholder="https://example.bt"
          helperText="Optional. A full http or https address." onChange={e => update('website', e.target.value)} />
        <TextInput label="Location" value={form.location} disabled={busy}
          helperText={`Optional. ${length(form.location)} of 100 characters.`} onChange={e => update('location', e.target.value)} />
        {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
        {error && <Alert severity="error">{error}</Alert>}
        {profile && !changed && <Typography variant="body2" color="text.secondary">No changes to save.</Typography>}
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
          <Button type="submit" variant="contained" disabled={busy || !ready}>{busy ? 'Saving…' : profile ? 'Save profile' : 'Create profile'}</Button>
          {profile && <Button disabled={busy} onClick={cancel}>Cancel</Button>}
        </Stack>
      </Stack>
    </FormPanel>}
    {profile && <CompanyLogoUpload profile={profile} onUploaded={setProfile} />}
    {profile
      ? <VerificationSection profile={profile} onRefreshProfile={() => setRevision(n => n + 1)} />
      : <Card component="section" label="Verification">
        <Typography variant="h2" component="h2" sx={{ mb: 1 }}>Verification</Typography>
        <Typography color="text.secondary">Create your company profile first, then request verification here.</Typography>
      </Card>}
  </Stack>;
}
