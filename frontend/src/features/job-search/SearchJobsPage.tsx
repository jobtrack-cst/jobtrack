import { useState } from 'react';
import { Alert, Autocomplete, Button, Card, CardContent, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { Link } from 'react-router';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { alertReferences } from '../job-alert/api';
import { SaveJobButton } from '../saved-jobs/SaveJobButton';
import { emptyFilters, searchJobs } from './api';

const messages = { 3: 'The search filters were not accepted. Check your filters and try again.' };
export function SearchJobsPage() {
  const [filters, setFilters] = useState(emptyFilters);
  const [draft, setDraft] = useState(emptyFilters);
  const [token, setToken] = useState('');
  const resource = useLoad(() => searchJobs(token, filters), [token, filters], messages);
  const refs = useLoad(alertReferences, [], messages);
  const invalid = length(draft.query) > 150 ? 'Search title must contain at most 150 characters.'
    : length(draft.location) > 100 ? 'Location must contain at most 100 characters.' : '';
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Search jobs</Typography>
    <Typography>Open jobs, soonest deadline first. Filters combine with AND; tags match any selected tag.</Typography>
    <ResourceState {...refs} />{refs.error && <Button onClick={refs.refresh}>Retry search options</Button>}
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); if (!invalid) { setToken(''); setFilters({ ...draft, query: draft.query.trim(), location: draft.location.trim() }); } }}>
      <TextField label="Search title" value={draft.query} onChange={e => setDraft({ ...draft, query: e.target.value })} />
      <TextField select label="Category" value={draft.categoryId} disabled={!refs.data} onChange={e => setDraft({ ...draft, categoryId: e.target.value })}>
        <MenuItem value="">Any</MenuItem>{refs.data?.categories.map(c => <MenuItem key={c.categoryId} value={c.categoryId}>{c.name}</MenuItem>)}
      </TextField>
      <Autocomplete multiple options={refs.data?.tags ?? []} value={refs.data?.tags.filter(t => draft.tagIds.includes(t.tagId)) ?? []}
        getOptionLabel={t => t.name} isOptionEqualToValue={(a, b) => a.tagId === b.tagId} disabled={!refs.data}
        onChange={(_, tags) => setDraft({ ...draft, tagIds: tags.map(t => t.tagId) })} renderInput={params => <TextField {...params} label="Tags" />} />
      <Autocomplete freeSolo options={refs.data?.locations.map(l => l.name) ?? []} inputValue={draft.location}
        onInputChange={(_, location) => setDraft({ ...draft, location })} renderInput={params => <TextField {...params} label="Location" />} />
      {invalid && <Alert severity="info">{invalid}</Alert>}
      <Button type="submit" disabled={resource.loading || !!invalid}>Search</Button>
      <Button onClick={() => { setDraft(emptyFilters); setFilters({ ...emptyFilters }); setToken(''); }}>Clear filters</Button>
    </Stack>
    <ResourceState {...resource} /><Button disabled={resource.loading} onClick={resource.refresh}>Refresh search</Button>
    {resource.data && <Typography>Matching jobs: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.jobs?.length && <Typography>No open jobs match these filters.</Typography>}
    {resource.data?.jobs?.map(job => <Card key={job.jobId}><CardContent><Stack spacing={1}>
      <Typography variant="h6">{job.title}</Typography><Typography>{job.location}</Typography>
      <Typography>Deadline: {formatTime(job.deadline)}</Typography>
      <Button component={Link} to={`/applicant/jobs/${job.jobId}`}>View job</Button><SaveJobButton jobId={job.jobId} />
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={resource.loading} onPage={setToken} />
  </Stack>;
}
