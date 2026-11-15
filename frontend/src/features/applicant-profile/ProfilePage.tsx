import { useEffect, useState } from 'react';
import { Alert, Autocomplete, Button, Card, CardContent, Chip, CircularProgress, Stack, TextField, Typography } from '@mui/material';
import { gatewayMessage, grpcCode, isNotFound, length } from '../application-status/shared';
import { createMyProfile, getMyProfile, updateMyProfile, type Education, type ProfileInput } from './api';

// JT-50 limits, checked here so the user sees the problem before saving.
const limits = { summary: 2000, education: 20, text: 150, skills: 50, skill: 50, firstYear: 1950 };
interface EducationRow { institution: string; qualification: string; startYear: string; endYear: string }
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

const toRow = (e: Education): EducationRow => ({ institution: e.institution, qualification: e.qualification, startYear: String(e.startYear), endYear: e.endYear ? String(e.endYear) : '' });
const toInput = (summary: string, rows: EducationRow[], skills: string[]): ProfileInput => ({
  summary: summary.trim(),
  education: rows.map(r => ({ institution: r.institution.trim(), qualification: r.qualification.trim(), startYear: Number(r.startYear), endYear: r.endYear.trim() === '' ? 0 : Number(r.endYear) })),
  skills: skills.map(s => s.trim()),
});
const saveMessages = {
  [grpcCode.invalidArgument]: 'The profile was not saved. Check the summary, education entries and skills against the limits shown and try again.',
  [grpcCode.alreadyExists]: 'You already have a profile. Reload the page to edit it.',
  [grpcCode.notFound]: 'Your profile was not found. Reload the page to create it.',
  [grpcCode.permissionDenied]: 'Only applicants can have a profile.',
};

export function ProfilePage() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [exists, setExists] = useState(false);
  const [summary, setSummary] = useState('');
  const [rows, setRows] = useState<EducationRow[]>([]);
  const [skills, setSkills] = useState<string[]>([]);
  const [revision, setRevision] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  useEffect(() => {
    let active = true;
    setLoading(true); setLoadError(''); setNotice(''); setError('');
    getMyProfile().then(({ profile }) => {
      if (!active) return;
      setExists(true); setSummary(profile.summary ?? ''); setRows((profile.education ?? []).map(toRow)); setSkills(profile.skills ?? []);
    }, reason => {
      if (!active) return;
      if (isNotFound(reason)) { setExists(false); setSummary(''); setRows([]); setSkills([]); }
      else setLoadError(gatewayMessage(reason, { [grpcCode.permissionDenied]: saveMessages[grpcCode.permissionDenied] }));
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [revision]);
  const problem = validateProfile(summary, rows, skills);
  async function save() {
    if (busy || problem) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const { profile } = await (exists ? updateMyProfile : createMyProfile)(toInput(summary, rows, skills));
      setExists(true); setSummary(profile.summary ?? ''); setRows((profile.education ?? []).map(toRow)); setSkills(profile.skills ?? []);
      setNotice(exists ? 'Profile updated.' : 'Profile created.');
    } catch (reason) { setError(gatewayMessage(reason, saveMessages)); }
    finally { setBusy(false); }
  }
  const setRow = (index: number, change: Partial<EducationRow>) => setRows(current => current.map((row, i) => i === index ? { ...row, ...change } : row));
  if (loading) return <CircularProgress aria-label="Loading" size={24} />;
  if (loadError) return <Stack spacing={2}><Alert severity="error">{loadError}</Alert><Button onClick={() => setRevision(n => n + 1)}>Try again</Button></Stack>;
  return <Stack spacing={2} component="form" onSubmit={e => { e.preventDefault(); void save(); }}>
    <Typography variant="h4" component="h1">My profile</Typography>
    {!exists && <Alert severity="info">You do not have a profile yet. Fill in the details and create it.</Alert>}
    <TextField label="Summary" name="summary" multiline minRows={3} value={summary} disabled={busy}
      helperText={`${length(summary)} of ${limits.summary} characters`} onChange={e => setSummary(e.target.value)} />
    <Typography variant="h6" component="h2">Education</Typography>
    {rows.length === 0 && <Typography>No education entries.</Typography>}
    {rows.map((row, index) => <Card key={index} variant="outlined"><CardContent><Stack spacing={2}>
      <Typography variant="subtitle1" component="h3">Education entry {index + 1}</Typography>
      <TextField label="Institution" required value={row.institution} disabled={busy} onChange={e => setRow(index, { institution: e.target.value })} />
      <TextField label="Qualification" required value={row.qualification} disabled={busy} onChange={e => setRow(index, { qualification: e.target.value })} />
      <Stack direction="row" spacing={2}>
        <TextField label="Start year" required value={row.startYear} disabled={busy} slotProps={{ htmlInput: { inputMode: 'numeric' } }} onChange={e => setRow(index, { startYear: e.target.value.trim() })} />
        <TextField label="End year" value={row.endYear} disabled={busy} helperText="Leave empty if ongoing" slotProps={{ htmlInput: { inputMode: 'numeric' } }} onChange={e => setRow(index, { endYear: e.target.value.trim() })} />
      </Stack>
      <Button disabled={busy} onClick={() => setRows(current => current.filter((_, i) => i !== index))}>Remove entry {index + 1}</Button>
    </Stack></CardContent></Card>)}
    <Button disabled={busy || rows.length >= limits.education} onClick={() => setRows(current => [...current, { ...emptyRow }])}>Add education</Button>
    <Autocomplete multiple freeSolo options={[]} value={skills} disabled={busy}
      onChange={(_, value) => setSkills(value.map(String))}
      renderValue={(value, getItemProps) => value.map((skill, index) => { const { key, ...props } = getItemProps({ index }); return <Chip key={key} label={skill} {...props} />; })}
      renderInput={params => <TextField {...params} label="Skills" helperText={`Type a skill and press Enter. ${skills.length} of ${limits.skills}.`} />} />
    {problem && <Alert severity="warning">{problem}</Alert>}
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    <Button type="submit" variant="contained" disabled={busy || !!problem}>{exists ? 'Save profile' : 'Create profile'}</Button>
  </Stack>;
}
