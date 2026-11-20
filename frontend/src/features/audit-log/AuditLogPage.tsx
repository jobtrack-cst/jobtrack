import { EmptyState } from '../../components/EmptyState';
import { useState } from 'react';
import { Alert, Box, Button, Card, CardContent, Stack, TextField, Typography } from '@mui/material';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listAuditEntries, type AuditFilters } from './api';

const messages = {
  [grpcCode.invalidArgument]: 'The filters were not accepted. Use a lowercase event type with dots, for example application.submitted, and a From time before the To time.',
  [grpcCode.permissionDenied]: 'Only admins can view the audit log.',
};
// JT-35: a lowercase routing key such as job_alert.matched.
const eventTypePattern = /^[a-z0-9_]+(\.[a-z0-9_]+)*$/;
// A datetime-local value is read in the browser's time zone.
function localTime(value: string) {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

export function AuditLogPage() {
  const [eventType, setEventType] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [applied, setApplied] = useState<AuditFilters>({ eventType: '' });
  const [token, setToken] = useState('');
  const list = useLoad(() => listAuditEntries(token, applied), [token, applied], messages);
  const type = eventType.trim();
  const start = localTime(from);
  const end = localTime(to);
  const problems: string[] = [];
  if (length(type) > 100) problems.push('Event type can have at most 100 characters.');
  else if (type && !eventTypePattern.test(type)) problems.push('Event type must be lowercase words joined by dots, for example application.submitted.');
  if (start && end && start >= end) problems.push('The From time must be before the To time.');
  function apply() {
    if (problems.length) return;
    setApplied({ eventType: type, startTime: start?.toISOString(), endTime: end?.toISOString() });
    setToken('');
  }
  function clear() { setEventType(''); setFrom(''); setTo(''); setApplied({ eventType: '' }); setToken(''); }
  const total = list.data?.pagination?.totalCount;
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Audit log</Typography>
    <Typography>Every domain event, newest first. The data is shown exactly as it was received.</Typography>
    <Stack component="form" direction={{ xs: 'column', xl: 'row' }} spacing={2} noValidate onSubmit={e => { e.preventDefault(); apply(); }}>
      <TextField label="Event type" value={eventType} placeholder="application.submitted" helperText="Optional"
        onChange={e => setEventType(e.target.value)} />
      <TextField label="From" type="datetime-local" value={from} helperText="Optional, included" slotProps={{ inputLabel: { shrink: true } }}
        onChange={e => setFrom(e.target.value)} />
      <TextField label="To" type="datetime-local" value={to} helperText="Optional, not included" slotProps={{ inputLabel: { shrink: true } }}
        onChange={e => setTo(e.target.value)} />
      <Button type="submit" variant="contained" disabled={list.loading || problems.length > 0}>Apply filters</Button>
      <Button disabled={list.loading} onClick={clear}>Clear filters</Button>
    </Stack>
    {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
    <Button disabled={list.loading} onClick={list.refresh}>Refresh entries</Button>
    {list.loading && <ResourceState loading error="" />}
    {!list.loading && list.error && <><Alert severity="error">{list.error}</Alert><Button onClick={list.refresh}>Try again</Button></>}
    {list.data && total !== undefined && <Typography>Matching entries: {Number(total).toLocaleString()}</Typography>}
    {list.data && !list.data.entries?.length && <EmptyState>No entries on this page.</EmptyState>}
    {list.data?.entries?.map(e => <Card key={e.eventId}><CardContent><Stack spacing={1}>
      <Typography variant="h6" component="h2">{e.eventType || 'Unknown event'}</Typography>
      <Typography>Producer: {e.producer || 'Unknown'}</Typography>
      <Typography>Occurred: {formatTime(e.occurredAt)}</Typography>
      <Typography className="record-id">Event ID: {e.eventId}</Typography>
      <Box component="pre" sx={{ m: 0, p: 1, bgcolor: '#edf1e6', borderRadius: '8px', border: '1px solid #dce3d4', whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 13 }}>{e.dataJson || '{}'}</Box>
    </Stack></CardContent></Card>)}
    <Pager next={list.data?.pagination?.nextPageToken} token={token} busy={list.loading} onPage={setToken} />
  </Stack>;
}
