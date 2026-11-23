import { useEffect, useState } from 'react';
import { Alert, Autocomplete, Box, Button, Chip, Stack, TextField, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { ErrorState } from '../../components/ErrorState';
import { FormPanel } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { TabBar, TabPanel, useTab } from '../../components/Tabs';
import { TextInput } from '../../components/TextInput';
import { tokens } from '../../theme';
import { gatewayMessage, grpcCode, isNotFound, length } from '../application-status/shared';
import { useAuth } from '../auth/AuthProvider';
import { ResumesPanel } from '../resume/ResumesPanel';
import { createMyProfile, getMyProfile, updateMyProfile, type ApplicantProfile, type ProfileInput } from './api';
import { ProfilePhoto } from './ProfilePhoto';
import { ProfilePhotoUpload } from './ProfilePhotoUpload';
import { profileValues, ProfileView, type EducationRow, type ProfileValues } from './ProfileView';

// JT-50 limits, checked here so the user sees the problem before saving.
const limits = { summary: 2000, education: 20, text: 150, skills: 50, skill: 50, firstYear: 1950 };
const emptyRow: EducationRow = { institution: '', qualification: '', startYear: '', endYear: '' };
const oneLine = (value: string) => !/[\r\n\t]/.test(value);

export function validateProfile(summary: string, rows: EducationRow[], skills: string[], year = new Date().getFullYear()): string {
  if (length(summary) > limits.summary) return `Summary can have at most ${limits.summary} characters.`;
  if (rows.length > limits.education) return `You can add at most ${limits.education} education entries.`;
  for (const [index, row] of rows.entries()) {
    const entry = `Education entry ${index + 1}`;
    for (const [label, value] of [['institution', row.institution], ['qualification', row.qualification]] as const) {
      if (length(value) < 1 || length(value) > limits.text || !oneLine(value)) return `${entry}: ${label} needs 1 to ${limits.text} characters on one line.`;
    }
    const start = Number(row.startYear);
    if (!/^\d{4}$/.test(row.startYear) || start < limits.firstYear || start > year + 1) return `${entry}: start year must be from ${limits.firstYear} to ${year + 1}.`;
    const end = row.endYear.trim() === '' ? 0 : Number(row.endYear);
    if (end !== 0 && (!/^\d{4}$/.test(row.endYear) || end < start || end > year + 10)) return `${entry}: end year must be empty (ongoing) or from ${start} to ${year + 10}.`;
  }
  if (skills.length > limits.skills) return `You can add at most ${limits.skills} skills.`;
  const seen = new Set<string>();
  for (const skill of skills) {
    if (length(skill) < 1 || length(skill) > limits.skill || !oneLine(skill)) return `Each skill needs 1 to ${limits.skill} characters on one line.`;
    const key = skill.trim().toLowerCase();
    if (seen.has(key)) return `The skill "${skill.trim()}" is listed twice.`;
    seen.add(key);
  }
  return '';
}

const toInput = (summary: string, rows: EducationRow[], skills: string[]): ProfileInput => ({
  summary: summary.trim(),
  education: rows.map(r => ({ institution: r.institution.trim(), qualification: r.qualification.trim(), startYear: Number(r.startYear), endYear: r.endYear.trim() === '' ? 0 : Number(r.endYear) })),
  skills: skills.map(s => s.trim()),
});
// The saved values, kept so Cancel can restore them and Save can tell whether anything changed (JT-109).
type Saved = ProfileValues;
const emptySaved: Saved = { summary: '', rows: [], skills: [] };
const sameInput = (a: ProfileInput, b: ProfileInput) => JSON.stringify(a) === JSON.stringify(b);

const saveMessages = {
  [grpcCode.invalidArgument]: 'The profile was not saved. Check the summary, education entries and skills against the limits shown and try again.',
  [grpcCode.alreadyExists]: 'You already have a profile. Reload the page to edit it.',
  [grpcCode.notFound]: 'Your profile was not found. Reload the page to create it.',
  [grpcCode.permissionDenied]: 'Only applicants can have a profile.',
};

const tabs = ['profile', 'resumes'] as const;

// Profile page with two tabs: the profile (read-only with Edit) and the resumes.
export function ProfilePage() {
  const [tab, setTab] = useTab(tabs);
  return <Stack spacing={4}>
    <PageHeader title="Profile" description="What employers read when you apply, and the resumes you apply with." />
    <Stack spacing={3}>
      <TabBar id="profile" label="Profile sections" value={tab} onChange={setTab} tabs={[['profile', 'Profile'], ['resumes', 'Resumes']]} />
      <TabPanel id="profile" value={tab}>{tab === 'resumes' ? <ResumesPanel /> : <ProfileTab />}</TabPanel>
    </Stack>
  </Stack>;
}

function ProfileTab() {
  const { session } = useAuth();
  const [hasPhoto, setHasPhoto] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [exists, setExists] = useState(false);
  const [summary, setSummary] = useState('');
  const [rows, setRows] = useState<EducationRow[]>([]);
  const [skills, setSkills] = useState<string[]>([]);
  const [saved, setSaved] = useState<Saved>(emptySaved);
  const [editing, setEditing] = useState(false);
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  // Shows the given values in the form and remembers them as saved.
  function show(next: Saved) {
    setSaved(next); setSummary(next.summary); setRows(next.rows); setSkills(next.skills);
  }
  const fromProfile = (profile: ApplicantProfile): Saved => profileValues(profile);
  useEffect(() => {
    let active = true;
    setLoading(true); setLoadError(''); setNotice(''); setError(''); setEditing(false);
    getMyProfile().then(({ profile }) => {
      if (!active) return;
      setExists(true); show(fromProfile(profile));
      setHasPhoto(profile.hasPhoto === true);
    }, reason => {
      if (!active) return;
      if (isNotFound(reason)) { setExists(false); show(emptySaved); setHasPhoto(false); }
      else setLoadError(gatewayMessage(reason, { [grpcCode.permissionDenied]: saveMessages[grpcCode.permissionDenied] }));
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [revision]);
  const problem = validateProfile(summary, rows, skills);
  // A new profile can always be created; an existing one only when something changed.
  const changed = !exists || !sameInput(toInput(summary, rows, skills), toInput(saved.summary, saved.rows, saved.skills));
  async function save() {
    if (busy || problem || !changed) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const { profile } = await (exists ? updateMyProfile : createMyProfile)(toInput(summary, rows, skills));
      setExists(true); show(fromProfile(profile)); setEditing(false);
      setHasPhoto(profile.hasPhoto === true);
      setNotice(exists ? 'Profile updated.' : 'Profile created.');
    } catch (reason) { setError(gatewayMessage(reason, saveMessages)); }
    finally { setBusy(false); }
  }
  function edit() { setNotice(''); setError(''); setEditing(true); }
  function cancel() { show(saved); setError(''); setEditing(false); }
  const setRow = (index: number, change: Partial<EducationRow>) => setRows(current => current.map((row, i) => i === index ? { ...row, ...change } : row));
  if (loading) return <LoadingRows rows={2} height={160} label="Loading profile" />;
  if (loadError) return <ErrorState message={loadError} onRetry={() => setRevision(n => n + 1)} />;
  return <Stack spacing={3}>
    {/* The upload only changes the photo and updated_at, so unsaved form edits are kept (JT-103).
        It needs a saved profile, so before that only the hint is shown. */}
    {exists && session
      ? <ProfilePhotoUpload userId={session.user.userId} name={session.user.fullName} hasPhoto={hasPhoto} onUploaded={profile => setHasPhoto(profile.hasPhoto === true)} />
      : <Card component="section"><Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        {session && <ProfilePhoto userId={session.user.userId} name={session.user.fullName} hasPhoto={false} size={56} />}
        <Box><Typography variant="h3" component="h2">Profile photo</Typography>
          <Typography color="text.secondary">Save your profile to add a photo.</Typography></Box>
      </Stack></Card>}
    {!exists && <Alert severity="info">You do not have a profile yet. Fill in the details and create it.</Alert>}
    {notice && <Alert severity="success">{notice}</Alert>}
    {exists && !editing && <FormPanel>
      <Stack spacing={3}>
        <ProfileView saved={saved} />
        <Button variant="contained" onClick={edit} sx={{ alignSelf: 'flex-start' }}>Edit profile</Button>
      </Stack>
    </FormPanel>}
    {(!exists || editing) && <FormPanel><Stack spacing={3} component="form" noValidate onSubmit={e => { e.preventDefault(); void save(); }}>
      <TextInput label="Summary" name="summary" multiline minRows={4} value={summary} disabled={busy}
        helperText={`${length(summary)} of ${limits.summary} characters`} onChange={e => setSummary(e.target.value)} />
      <Stack spacing={2}>
        <Typography variant="h2" component="h2">Education</Typography>
        {rows.length === 0 && <Typography color="text.secondary">No education entries.</Typography>}
        {rows.map((row, index) => <Box key={index} component="fieldset" sx={{ m: 0, p: { xs: 2, sm: 2.5 }, border: `1px solid ${tokens.border}`, borderRadius: '16px', minWidth: 0 }}>
          <Typography component="legend" sx={{ px: 1, fontWeight: 700 }}>Education entry {index + 1}</Typography>
          <Stack spacing={2}>
            <TextInput label="Institution" required value={row.institution} disabled={busy} onChange={e => setRow(index, { institution: e.target.value })} />
            <TextInput label="Qualification" required value={row.qualification} disabled={busy} onChange={e => setRow(index, { qualification: e.target.value })} />
            <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: 'minmax(0, 1fr)', sm: 'repeat(2, minmax(0, 1fr))' } }}>
              <TextInput label="Start year" required value={row.startYear} disabled={busy} inputMode="numeric" onChange={e => setRow(index, { startYear: e.target.value.trim() })} />
              <TextInput label="End year" value={row.endYear} disabled={busy} inputMode="numeric" helperText="Leave empty if ongoing" onChange={e => setRow(index, { endYear: e.target.value.trim() })} />
            </Box>
            <Button color="error" disabled={busy} onClick={() => setRows(current => current.filter((_, i) => i !== index))} sx={{ alignSelf: 'flex-start' }}>Remove entry {index + 1}</Button>
          </Stack>
        </Box>)}
        <Button variant="outlined" disabled={busy || rows.length >= limits.education} onClick={() => setRows(current => [...current, { ...emptyRow }])} sx={{ alignSelf: 'flex-start' }}>Add education</Button>
      </Stack>
      <Autocomplete multiple freeSolo options={[]} value={skills} disabled={busy}
        onChange={(_, value) => setSkills(value.map(String))}
        renderValue={(value, getItemProps) => value.map((skill, index) => { const { key, ...props } = getItemProps({ index }); return <Chip key={key} label={skill} {...props} />; })}
        renderInput={params => <TextField {...params} label="Skills" helperText={`Type a skill and press Enter. ${skills.length} of ${limits.skills}.`} />} />
      {problem && <Alert severity="warning">{problem}</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      {exists && !changed && <Typography variant="body2" color="text.secondary">No changes to save.</Typography>}
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
        <Button type="submit" variant="contained" disabled={busy || !!problem || !changed}>{exists ? 'Save profile' : 'Create profile'}</Button>
        {exists && <Button disabled={busy} onClick={cancel}>Cancel</Button>}
      </Stack>
    </Stack></FormPanel>}
  </Stack>;
}
