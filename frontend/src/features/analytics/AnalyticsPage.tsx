import { useState } from 'react';
import { Alert, Button, CircularProgress, Stack, Table, TableBody, TableCell, TableHead, TableRow, TextField, Typography } from '@mui/material';
import { formatTime, gatewayMessage, grpcCode } from '../application-status/shared';
import { getSummary, type Summary } from './api';
import { dayInput, dayStart } from './dates';

const messages = {
  [grpcCode.invalidArgument]: 'The period was not accepted. Choose a From date on or before the To date.',
  [grpcCode.permissionDenied]: 'Only admins can view analytics.',
};
type CountKey = Exclude<keyof Summary, 'startTime' | 'endTime' | 'statusChanges'>;
const rows: [CountKey, string][] = [
  ['jobsPublished', 'Jobs published'], ['jobsClosed', 'Jobs closed'],
  ['applicationsSubmitted', 'Applications submitted'], ['applicationsWithdrawn', 'Applications withdrawn'],
  ['offersMade', 'Offers made'], ['offersAccepted', 'Offers accepted'], ['offersDeclined', 'Offers declined'],
];
const count = (value?: string | number) => Number(value ?? 0).toLocaleString();
// Events carry names such as "UNDER_REVIEW"; show them as "Under review".
function statusName(status?: string) {
  const name = (status ?? '').replace(/^APPLICATION_STATUS_/, '');
  return name ? name.charAt(0) + name.slice(1).toLowerCase().replace(/_/g, ' ') : 'Unknown';
}

export function AnalyticsPage() {
  const [from, setFrom] = useState(() => { const d = new Date(); return dayInput(new Date(d.getFullYear(), d.getMonth(), d.getDate() - 6)); });
  const [to, setTo] = useState(() => dayInput(new Date()));
  const [summary, setSummary] = useState<Summary>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  // The To day is included, so the period ends at the next local midnight.
  const start = dayStart(from);
  const end = dayStart(to, 1);
  const problem = !start || !end ? 'Choose both dates.' : end <= start ? 'The To date must be on or after the From date.' : '';
  async function load() {
    if (busy || !start || !end || problem) return;
    setBusy(true); setError(''); setSummary(undefined);
    try { setSummary((await getSummary(start.toISOString(), end.toISOString())).summary ?? {}); }
    catch (failure) { setError(gatewayMessage(failure, messages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Analytics</Typography>
    <Typography>Counts events from the start of the From day to the end of the To day, in your time zone.</Typography>
    <Stack component="form" direction={{ xs: 'column', sm: 'row' }} spacing={2} noValidate onSubmit={e => { e.preventDefault(); void load(); }}>
      <TextField label="From" type="date" required value={from} disabled={busy} slotProps={{ inputLabel: { shrink: true } }}
        onChange={e => setFrom(e.target.value)} />
      <TextField label="To" type="date" required value={to} disabled={busy} slotProps={{ inputLabel: { shrink: true } }}
        onChange={e => setTo(e.target.value)} />
      <Button type="submit" variant="contained" disabled={busy || !!problem}>Show summary</Button>
    </Stack>
    {problem && <Alert severity="warning">{problem}</Alert>}
    {busy && <CircularProgress aria-label="Loading" size={24} />}
    {error && <Alert severity="error">{error}</Alert>}
    {summary && <Stack spacing={2}>
      <Typography>Period: {formatTime(summary.startTime)} to {formatTime(summary.endTime)} (end not included)</Typography>
      <Table size="small" aria-label="Outcome counts"><TableBody>
        {rows.map(([key, label]) => <TableRow key={key}>
          <TableCell component="th" scope="row">{label}</TableCell><TableCell align="right">{count(summary[key])}</TableCell>
        </TableRow>)}
      </TableBody></Table>
      <Typography variant="h6" component="h2">Application status changes</Typography>
      {summary.statusChanges?.length
        ? <Table size="small" aria-label="Status changes">
          <TableHead><TableRow><TableCell>New status</TableCell><TableCell align="right">Count</TableCell></TableRow></TableHead>
          <TableBody>{summary.statusChanges.map(s => <TableRow key={s.status ?? ''}>
            <TableCell>{statusName(s.status)}</TableCell><TableCell align="right">{count(s.count)}</TableCell>
          </TableRow>)}</TableBody>
        </Table>
        : <Typography>No status changes in this period.</Typography>}
    </Stack>}
  </Stack>;
}
