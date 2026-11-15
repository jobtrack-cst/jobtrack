import { useState } from 'react';
import { Alert, Autocomplete, Button, Card, CardContent, Dialog, DialogActions, DialogTitle, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { errorMessage, Pager, ResourceState, useResource } from '../application/shared';
import { alertReferences, createAlert, deleteAlert, listAlerts, type Category, type Tag } from './api';

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
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Job alert subscriptions</Typography>
    <Typography>Up to 10 subscriptions. Category, tag and location filters combine with AND. Empty filters match any value. To edit a subscription, delete it and create another.</Typography>
    <ResourceState {...resource} /><ResourceState {...refs} />
    <Button disabled={busy || resource.loading || refs.loading} onClick={() => { resource.refresh(); refs.refresh(); }}>Refresh subscriptions</Button>
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    {resource.data && !resource.data.jobAlerts?.length && <Typography>No subscriptions on this page.</Typography>}
    {resource.data?.jobAlerts?.map(a => <Card key={a.alertId}><CardContent><Stack spacing={1}>
      <Typography>Categories: {a.categoryIds?.map(id => refs.data?.categories.find(c => c.categoryId === id)?.name ?? id).join(', ') || 'Any'}</Typography>
      <Typography>Tags: {a.tagIds?.map(id => refs.data?.tags.find(t => t.tagId === id)?.name ?? id).join(', ') || 'Any'}</Typography>
      <Typography>Location: {a.location || 'Any'}</Typography><Button disabled={busy} onClick={() => setSelected(a.alertId)}>Delete subscription</Button>
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={busy || resource.loading} onPage={setToken} />
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); if (valid) void change(async () => { await createAlert(categories.map(c => c.categoryId), tags.map(t => t.tagId), location); setCategories([]); setTags([]); setLocation(''); }, 'Subscription created.'); }}>
      <Autocomplete multiple options={refs.data?.categories ?? []} value={categories} getOptionLabel={c => c.name} isOptionEqualToValue={(a, b) => a.categoryId === b.categoryId} disabled={busy || !refs.data} onChange={(_, value) => setCategories(value)} renderInput={params => <TextField {...params} label="Categories" helperText="Choose up to 20" />} />
      <Autocomplete multiple options={refs.data?.tags ?? []} value={tags} getOptionLabel={t => t.name} isOptionEqualToValue={(a, b) => a.tagId === b.tagId} disabled={busy || !refs.data} onChange={(_, value) => setTags(value)} renderInput={params => <TextField {...params} label="Tags" helperText="Choose up to 20" />} />
      <TextField select label="Location" value={location} disabled={busy || !refs.data} onChange={e => setLocation(e.target.value)} helperText="Uses the exact location name. Any leaves the location filter empty." slotProps={{ select: { displayEmpty: true }, inputLabel: { shrink: true } }}>
        <MenuItem value="">Any</MenuItem>
        {refs.data?.locations.map(l => <MenuItem key={l.locationId} value={l.name}>{l.name}</MenuItem>)}
      </TextField>
      <Button type="submit" disabled={busy || !valid}>Create subscription</Button>
    </Stack>
    <Dialog open={!!selected} onClose={() => { if (!busy) setSelected(''); }}><DialogTitle>Delete this subscription?</DialogTitle><DialogActions>
      <Button disabled={busy} onClick={() => setSelected('')}>Cancel</Button><Button disabled={busy} onClick={() => void change(() => deleteAlert(selected), 'Subscription deleted.')}>Confirm deletion</Button>
    </DialogActions></Dialog>
  </Stack>;
}
