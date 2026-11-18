import { useState } from 'react';
import { Alert, Autocomplete, Button, MenuItem, Stack, TextField } from '@mui/material';
import { ResourceState } from '../application/shared';
import { length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { alertReferences } from '../job-alert/api';
import type { Job, JobInput } from './api';
import { jobMessages } from './shared';

const localTime = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
};
export function JobForm({ job, busy, onSave }: { job?: Job; busy: boolean; onSave: (input: JobInput) => Promise<void> }) {
  const refs = useLoad(alertReferences, [], jobMessages);
  const [title, setTitle] = useState(job?.title ?? '');
  const [description, setDescription] = useState(job?.description ?? '');
  const [categoryId, setCategoryId] = useState(job?.categoryId ?? '');
  const [tagIds, setTagIds] = useState<string[]>(job?.tagIds ?? []);
  const [location, setLocation] = useState(job?.location ?? '');
  const [deadline, setDeadline] = useState(localTime(job?.deadline));
  // Keep the original wire precision when only other fields are edited.
  const originalDeadline = deadline === localTime(job?.deadline) ? job?.deadline : undefined;
  const deadlineValue = originalDeadline ?? deadline;
  const invalid = !length(title) || length(title) > 150 ? 'Title must contain 1 to 150 characters.'
    : length(description) > 5000 ? 'Description must contain at most 5000 characters.'
    : length(location) > 100 ? 'Location must contain at most 100 characters.'
    : tagIds.length > 20 ? 'Choose at most 20 tags.'
    : [title, description, location].some(value => value.includes('\0')) ? 'Text must not contain NUL characters.'
    : !deadlineValue || !Number.isFinite(Date.parse(deadlineValue)) || Date.parse(deadlineValue) <= Date.now() ? 'Choose a future deadline in your local time.' : '';
  return <Stack component="form" spacing={2} onSubmit={e => {
    e.preventDefault();
    if (!invalid && refs.data && !busy) void onSave({ title: title.trim(), description: description.trim(), categoryId, tagIds, location: location.trim(), deadline: originalDeadline ?? new Date(deadline).toISOString() });
  }}>
    <ResourceState {...refs} />
    {refs.error && <Button onClick={refs.refresh}>Retry job options</Button>}
    <TextField label="Title" required value={title} disabled={busy} onChange={e => setTitle(e.target.value)} />
    <TextField label="Description" multiline minRows={3} value={description} disabled={busy} onChange={e => setDescription(e.target.value)} />
    <TextField select label="Category" value={categoryId} disabled={busy || !refs.data} onChange={e => setCategoryId(e.target.value)}>
      <MenuItem value="">None</MenuItem>
      {categoryId && !refs.data?.categories.some(c => c.categoryId === categoryId) && <MenuItem value={categoryId}>Current category ({categoryId})</MenuItem>}
      {refs.data?.categories.map(c => <MenuItem key={c.categoryId} value={c.categoryId}>{c.name}</MenuItem>)}
    </TextField>
    <Autocomplete multiple options={[...new Set([...(refs.data?.tags.map(t => t.tagId) ?? []), ...tagIds])]} value={tagIds}
      getOptionLabel={id => refs.data?.tags.find(t => t.tagId === id)?.name ?? id} disabled={busy || !refs.data}
      onChange={(_, ids) => setTagIds(ids)} renderInput={params => <TextField {...params} label="Tags" helperText="At most 20 tags" />} />
    <Autocomplete freeSolo options={refs.data?.locations.map(l => l.name) ?? []} inputValue={location} disabled={busy || !refs.data}
      onInputChange={(_, value) => setLocation(value)} renderInput={params => <TextField {...params} label="Location" />} />
    <TextField label="Deadline" type="datetime-local" value={deadline} disabled={busy} onChange={e => setDeadline(e.target.value)} slotProps={{ inputLabel: { shrink: true } }} helperText="Your local time; sent in UTC." required />
    {invalid && <Alert severity="info">{invalid}</Alert>}
    <Button type="submit" variant="contained" disabled={busy || !refs.data || !!invalid}>{job ? 'Save changes' : 'Create draft'}</Button>
  </Stack>;
}
