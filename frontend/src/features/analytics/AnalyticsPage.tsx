import { useState } from 'react';
import { Alert, Box, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { EmptyState } from '../../components/EmptyState';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { SearchBar, SearchField } from '../../components/SearchBar';
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
const highlights: [CountKey, string][] = [['jobsPublished', 'Jobs published'], ['applicationsSubmitted', 'Applications submitted'], ['offersAccepted', 'Offers accepted']];
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
  return <Stack spacing={4}>
    <PageHeader title="Analytics" description="Counts events from the start of the From day to the end of the To day, in your time zone." />
    <SearchBar label="Analytics period" submitLabel="Show summary" submitDisabled={busy || !!problem} onSubmit={() => void load()}>
      <SearchField label="From" type="date" value={from} disabled={busy} onChange={setFrom} />
      <SearchField label="To" type="date" value={to} disabled={busy} onChange={setTo} />
    </SearchBar>
    {problem && <Alert severity="warning">{problem}</Alert>}
    {busy && <LoadingRows rows={1} label="Loading summary" />}
    {error && <Alert severity="error">{error}</Alert>}
    {summary && <Stack spacing={4}>
      <Typography variant="body2" color="text.secondary">Period: {formatTime(summary.startTime)} to {formatTime(summary.endTime)} (end not included)</Typography>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', sm: 'repeat(3, minmax(0, 1fr))' }, gap: 2 }}>
        {highlights.map(([key, label]) => <Card key={key}>
          <Typography color="text.secondary" sx={{ fontWeight: 600 }}>{label}</Typography>
          <Typography sx={{ mt: 0.5, fontSize: '2.5rem', letterSpacing: '-0.04em', lineHeight: 1.2, fontWeight: 800, fontVariantNumeric: 'tabular-nums' }}>{count(summary[key])}</Typography>
        </Card>)}
      </Box>
      <Typography variant="h2">Outcome counts</Typography>
      <Table size="small" aria-label="Outcome counts"><TableHead><TableRow><TableCell>Event</TableCell><TableCell align="right">Count</TableCell></TableRow></TableHead><TableBody>
        {rows.map(([key, label]) => <TableRow key={key}>
          <TableCell component="th" scope="row">{label}</TableCell><TableCell align="right">{count(summary[key])}</TableCell>
        </TableRow>)}
      </TableBody></Table>
      <Typography variant="h2">Application status changes</Typography>
      {summary.statusChanges?.length
        ? <Table size="small" aria-label="Status changes">
          <TableHead><TableRow><TableCell>New status</TableCell><TableCell align="right">Count</TableCell></TableRow></TableHead>
          <TableBody>{summary.statusChanges.map(s => <TableRow key={s.status ?? ''}>
            <TableCell>{statusName(s.status)}</TableCell><TableCell align="right">{count(s.count)}</TableCell>
          </TableRow>)}</TableBody>
        </Table>
        : <EmptyState description="No status changes in this period." />}
    </Stack>}
  </Stack>;
}
