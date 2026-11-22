import { useId, useState } from 'react';
import { Alert, Box, Button, Collapse, Stack, Typography } from '@mui/material';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { Icon } from '../../components/Icon';
import { ListPanel, ListRow, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { SearchBar, SearchField } from '../../components/SearchBar';
import { Pager } from '../../components/Pager';
import { tokens } from '../../theme';
import { formatTime, grpcCode, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listAuditEntries, type AuditEntry, type AuditFilters } from './api';

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

// The payload is shown exactly as received (JT-35). This admin-only page is the
// one place IDs appear (redesign doc Section 4), and only after the admin expands it.
function EntryCard({ entry }: { entry: AuditEntry }) {
  const [open, setOpen] = useState(false);
  const regionId = useId();
  return <ListRow>
    <Stack spacing={1}>
      <Typography variant="h3" component="h2" sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace', fontSize: '1rem' }}>{entry.eventType || 'Unknown event'}</Typography>
      <Typography variant="body2" color="text.secondary">From {entry.producer || 'an unknown producer'} · {formatTime(entry.occurredAt)}</Typography>
      <Box>
        <Button aria-expanded={open} aria-controls={regionId} onClick={() => setOpen(o => !o)}
          endIcon={<Icon name="chevronDown" size={16} sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 180ms ease' }} />}>
          {open ? 'Hide event data' : 'Show event data'}
        </Button>
      </Box>
      <Collapse in={open} timeout={180} unmountOnExit>
        <Box id={regionId} role="region" aria-label="Event data">
          <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace', mb: 1 }}>Event ID: {entry.eventId}</Typography>
          <Box component="pre" tabIndex={0} sx={{ m: 0, p: 2, bgcolor: tokens.canvas, color: tokens.chipText, borderRadius: '12px', border: `1px solid ${tokens.border}`,
            whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: '0.8125rem', lineHeight: 1.6, fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace',
            maxHeight: 360, overflow: 'auto' }}>{entry.dataJson || '{}'}</Box>
        </Box>
      </Collapse>
    </Stack>
  </ListRow>;
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
  const entries = list.data?.entries ?? [];
  const total = Number(list.data?.pagination?.totalCount ?? entries.length);
  const filtered = !!(applied.eventType || applied.startTime || applied.endTime);
  return <Stack spacing={4}>
    <PageHeader title="Audit log" description="Every domain event, newest first. Event data is shown exactly as it was received."
      action={<Button variant="outlined" disabled={list.loading} onClick={list.refresh}>Refresh entries</Button>} />
    <Stack spacing={1.5}>
      <SearchBar label="Audit log filters" submitLabel="Apply filters" submitDisabled={list.loading || problems.length > 0} onSubmit={apply}>
        <SearchField label="Event type" value={eventType} placeholder="Any, for example application.submitted" onChange={setEventType} grow={1.4} />
        <SearchField label="From (included)" type="datetime-local" value={from} onChange={setFrom} />
        <SearchField label="To (not included)" type="datetime-local" value={to} onChange={setTo} />
      </SearchBar>
      {(filtered || eventType || from || to) && <Box><Button disabled={list.loading} onClick={clear}>Clear filters</Button></Box>}
    </Stack>
    {problems.map(p => <Alert key={p} severity="warning">{p}</Alert>)}
    {list.loading && <LoadingRows rows={3} label="Loading entries" />}
    {!list.loading && list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !entries.length && <EmptyState title="No entries" description="No events match these filters on this page." />}
    {!!entries.length && <Stack spacing={2}>
      <ResultsHeader title={`${total.toLocaleString()} ${total === 1 ? 'entry' : 'entries'}`} note="Newest first" />
      <ListPanel label="Audit entries">{entries.map(e => <EntryCard key={e.eventId} entry={e} />)}</ListPanel>
    </Stack>}
    <Pager token={token} nextToken={list.data?.pagination?.nextPageToken} totalCount={list.data?.pagination?.totalCount}
      count={entries.length} busy={list.loading} onPage={setToken} />
  </Stack>;
}
