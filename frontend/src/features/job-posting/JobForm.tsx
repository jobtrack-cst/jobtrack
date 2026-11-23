import { useState } from 'react';
import { Alert, Autocomplete, Button, Chip, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { TextInput } from '../../components/TextInput';
import { length } from '../application-status/shared';
import type { Category, Location, Tag } from '../job-alert/api';
import type { Job, JobInput } from './api';
import { tagName, unknownCategory } from './shared';

const localTime = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
};
const sameInput = (a: JobInput, b: JobInput) => JSON.stringify(a) === JSON.stringify(b);

interface Props {
  job?: Job;
  references: { categories: Category[]; tags: Tag[]; locations: Location[] };
  busy: boolean;
  onSave: (input: JobInput) => Promise<void>;
  // Shown for an existing job: returns to the read-only view (JT-109).
  onCancel?: () => void;
}

// Job editor (JT-38 limits). A category or tag that category-tag no longer lists is
// shown as "Unknown category" or "Unknown tag" and is sent back unchanged, so it is
// never cleared without the employer choosing to.
export function JobForm({ job, references, busy, onSave, onCancel }: Props) {
  const { categories, tags, locations } = references;
  const [title, setTitle] = useState(job?.title ?? '');
  const [description, setDescription] = useState(job?.description ?? '');
  const [categoryId, setCategoryId] = useState(job?.categoryId ?? '');
  const [tagIds, setTagIds] = useState<string[]>(job?.tagIds ?? []);
  const [location, setLocation] = useState(job?.location ?? '');
  const [deadline, setDeadline] = useState(localTime(job?.deadline));
  // Keep the original wire precision when only other fields are edited.
  const originalDeadline = deadline === localTime(job?.deadline) ? job?.deadline : undefined;
  const deadlineValue = originalDeadline ?? deadline;
  const unknownCategoryId = job?.categoryId && !categories.some(c => c.categoryId === job.categoryId) ? job.categoryId : '';
  const invalid = !length(title) || length(title) > 150 ? 'Title must contain 1 to 150 characters.'
    : length(description) > 5000 ? 'Description must contain at most 5000 characters.'
    : length(location) > 100 ? 'Location must contain at most 100 characters.'
    : tagIds.length > 20 ? 'Choose at most 20 tags.'
    : [title, description, location].some(value => value.includes('\0')) ? 'Text must not contain NUL characters.'
    : !deadlineValue || !Number.isFinite(Date.parse(deadlineValue)) || Date.parse(deadlineValue) <= Date.now() ? 'Choose a future deadline in your local time.' : '';
  const input = (): JobInput => ({ title: title.trim(), description: description.trim(), categoryId, tagIds, location: location.trim(),
    deadline: originalDeadline ?? (deadline ? new Date(deadline).toISOString() : '') });
  // A new job can always be created; an existing one only when something changed (JT-109).
  const changed = !job || invalid !== '' || !sameInput(input(), { title: job.title ?? '', description: job.description ?? '', categoryId: job.categoryId ?? '',
    tagIds: job.tagIds ?? [], location: job.location ?? '', deadline: job.deadline });
  return <Stack component="form" spacing={2.5} noValidate aria-label={job ? 'Edit job' : 'New job'} onSubmit={e => {
    e.preventDefault();
    if (!invalid && changed && !busy) void onSave(input());
  }}>
    <TextInput label="Title" required value={title} disabled={busy} helperText={`${length(title)} of 150 characters`} onChange={e => setTitle(e.target.value)} />
    <TextInput label="Description" multiline minRows={4} value={description} disabled={busy}
      helperText={`Optional. ${length(description)} of 5000 characters.`} onChange={e => setDescription(e.target.value)} />
    <TextInput label="Category" select value={categoryId} disabled={busy} onChange={e => setCategoryId(e.target.value)}>
      <MenuItem value="">No category</MenuItem>
      {unknownCategoryId && <MenuItem value={unknownCategoryId}>{unknownCategory}</MenuItem>}
      {categories.map(c => <MenuItem key={c.categoryId} value={c.categoryId}>{c.name}</MenuItem>)}
    </TextInput>
    <Autocomplete multiple options={[...new Set([...tags.map(t => t.tagId), ...tagIds])]} value={tagIds}
      getOptionLabel={id => tagName(tags, id)} disabled={busy} onChange={(_, ids) => setTagIds(ids)}
      renderValue={(value, getItemProps) => value.map((id, index) => { const { key, ...props } = getItemProps({ index }); return <Chip key={key} label={tagName(tags, id)} {...props} />; })}
      renderInput={params => <TextField {...params} label="Tags" helperText={`${tagIds.length} of 20 tags`} />} />
    <Autocomplete freeSolo options={locations.map(l => l.name)} inputValue={location} disabled={busy}
      onInputChange={(_, value) => setLocation(value)} renderInput={params => <TextField {...params} label="Location" helperText={`${length(location)} of 100 characters`} />} />
    <TextInput label="Deadline" type="datetime-local" required value={deadline} disabled={busy} helperText="Your local time; sent in UTC."
      onChange={e => setDeadline(e.target.value)} />
    {invalid && <Alert severity="info">{invalid}</Alert>}
    {job && !invalid && !changed && <Typography variant="body2" color="text.secondary">No changes to save.</Typography>}
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
      <Button type="submit" variant="contained" disabled={busy || !!invalid || !changed}>{busy ? 'Saving…' : job ? 'Save changes' : 'Create draft'}</Button>
      {onCancel && <Button disabled={busy} onClick={onCancel}>Cancel</Button>}
    </Stack>
  </Stack>;
}
