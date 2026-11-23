import { useState } from 'react';
import { Alert, Autocomplete, Box, Button, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { FormPanel, ListPanel, ListRow, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { errorMessage, useResource } from '../application/shared';
import { alertReferences, createAlert, deleteAlert, listAlerts, type Category, type JobAlert, type Tag } from './api';

function AlertRow({ alert, names, busy, onDelete }: {
  alert: JobAlert; names?: { categories: Category[]; tags: Tag[] }; busy: boolean; onDelete: () => void;
}) {
  // Names only; an ID that has no name yet is left out rather than shown.
  const categories = (alert.categoryIds ?? []).map(id => names?.categories.find(c => c.categoryId === id)?.name).filter((n): n is string => !!n);
  const tags = (alert.tagIds ?? []).map(id => names?.tags.find(t => t.tagId === id)?.name).filter((n): n is string => !!n);
  const line = (label: string, values: string[], given: number) => <Typography variant="body2" color="text.secondary">
    <Box component="span" sx={{ fontWeight: 600, color: 'text.primary' }}>{label}: </Box>
    {values.length ? values.join(', ') : given ? 'Loading names…' : 'Any'}
  </Typography>;
  return <ListRow>
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ justifyContent: 'space-between', alignItems: { sm: 'center' } }}>
      <Stack spacing={0.5} sx={{ minWidth: 0 }}>
        {line('Categories', categories, alert.categoryIds?.length ?? 0)}
        {line('Tags', tags, alert.tagIds?.length ?? 0)}
        {line('Location', alert.location ? [alert.location] : [], 0)}
      </Stack>
      <Button color="error" variant="outlined" disabled={busy} onClick={onDelete} sx={{ flexShrink: 0, alignSelf: { xs: 'flex-start', sm: 'center' } }}>Delete subscription</Button>
    </Stack>
  </ListRow>;
}

// Job alert subscriptions (JT-76 rules). Restyled in JT-112; the rules are unchanged.
export function JobAlertsPage() {
  const [token, setToken] = useState('');
  const resource = useResource(() => listAlerts(token), [token]);
  const refs = useResource(alertReferences, []);
  const [categories, setCategories] = useState<Category[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [location, setLocation] = useState('');
  const [selected, setSelected] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function change(action: () => Promise<unknown>, message: string) {
    if (busy) return;
    setBusy(true); setError(''); setNotice('');
    try { await action(); setSelected(''); setNotice(message); setToken(''); resource.refresh(); }
    catch (reason) { setError(errorMessage(reason, 'alert')); }
    finally { setBusy(false); }
  }
  const valid = categories.length <= 20 && tags.length <= 20 && Array.from(location).length <= 100 && !!(categories.length || tags.length || location);
  const alerts = resource.data?.jobAlerts ?? [];
  const total = Number(resource.data?.pagination?.totalCount ?? alerts.length);
  return <Stack spacing={4}>
    <PageHeader title="Job alerts" description="Hear about new jobs that match. Up to 10 subscriptions; to change one, delete it and create another." />
    {error && <Alert severity="error">{error}</Alert>}
    {notice && <Alert severity="success">{notice}</Alert>}
    <Stack component="section" aria-label="Subscriptions" spacing={2}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }}>
        <Box sx={{ flex: 1, minWidth: 0 }}><ResultsHeader title={resource.data ? `${total} ${total === 1 ? 'subscription' : 'subscriptions'}` : 'Subscriptions'} /></Box>
        <Button disabled={busy || resource.loading || refs.loading} onClick={() => { resource.refresh(); refs.refresh(); }}>Refresh subscriptions</Button>
      </Stack>
      {resource.loading && <LoadingRows rows={2} label="Loading subscriptions" />}
      {resource.error && <ErrorState message={resource.error} onRetry={resource.refresh} />}
      {resource.data && !alerts.length && <EmptyState title="No subscriptions" description="Create one below to hear about matching jobs." />}
      {!!alerts.length && <ListPanel label="Subscriptions">
        {alerts.map(a => <AlertRow key={a.alertId} alert={a} names={refs.data} busy={busy} onDelete={() => setSelected(a.alertId)} />)}
      </ListPanel>}
      {resource.data && <Pager token={token} nextToken={resource.data.pagination?.nextPageToken} totalCount={resource.data.pagination?.totalCount}
        count={alerts.length} busy={busy || resource.loading} onPage={setToken} />}
    </Stack>
    <FormPanel>
      <Stack component="form" spacing={2.5} onSubmit={e => { e.preventDefault(); if (valid) void change(async () => { await createAlert(categories.map(c => c.categoryId), tags.map(t => t.tagId), location); setCategories([]); setTags([]); setLocation(''); }, 'Subscription created.'); }}>
        <Typography variant="h2" component="h2">New subscription</Typography>
        <Typography color="text.secondary">Category, tag and location filters combine with AND. Empty filters match any value.</Typography>
        {refs.loading && <LoadingRows rows={1} label="Loading filters" />}
        {refs.error && <ErrorState message={refs.error} onRetry={refs.refresh} />}
        <Autocomplete multiple options={refs.data?.categories ?? []} value={categories} getOptionLabel={c => c.name} isOptionEqualToValue={(a, b) => a.categoryId === b.categoryId}
          disabled={busy || !refs.data} onChange={(_, value) => setCategories(value)}
          renderInput={params => <TextField {...params} label="Categories" helperText="Choose up to 20" />} />
        <Autocomplete multiple options={refs.data?.tags ?? []} value={tags} getOptionLabel={t => t.name} isOptionEqualToValue={(a, b) => a.tagId === b.tagId}
          disabled={busy || !refs.data} onChange={(_, value) => setTags(value)}
          renderInput={params => <TextField {...params} label="Tags" helperText="Choose up to 20" />} />
        <TextField select label="Location" value={location} disabled={busy || !refs.data} onChange={e => setLocation(e.target.value)}
          helperText="Uses the exact location name. Any leaves the location filter empty." slotProps={{ select: { displayEmpty: true }, inputLabel: { shrink: true } }}>
          <MenuItem value="">Any</MenuItem>
          {refs.data?.locations.map(l => <MenuItem key={l.locationId} value={l.name}>{l.name}</MenuItem>)}
        </TextField>
        <Button variant="contained" type="submit" disabled={busy || !valid} sx={{ alignSelf: 'flex-start' }}>Create subscription</Button>
      </Stack>
    </FormPanel>
    <ConfirmDialog open={!!selected} title="Delete this subscription?" confirmLabel="Confirm deletion" destructive busy={busy}
      onCancel={() => setSelected('')} onConfirm={() => void change(() => deleteAlert(selected), 'Subscription deleted.')}>
      <Typography>You will stop hearing about jobs that match it.</Typography>
    </ConfirmDialog>
  </Stack>;
}
