import { useState } from 'react';
import { Alert, Box, Button, Chip, Stack } from '@mui/material';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { JobCard } from '../../components/JobCard';
import { CardGrid, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { SearchBar, SearchField, SearchMultiSelect, SearchSelect } from '../../components/SearchBar';
import { tokens } from '../../theme';
import { length } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { alertReferences } from '../job-alert/api';
import type { Job } from '../job-posting/api';
import { SaveJobButton, useSaveJob } from '../saved-jobs/SaveJobButton';
import { emptyFilters, searchJobs, type SearchFilters } from './api';

const messages = { 3: 'The search filters were not accepted. Check your filters and try again.' };
type Result = Omit<Job, 'description' | 'status'>;

function ResultCard({ job, tags }: { job: Result; tags: string[] }) {
  const save = useSaveJob(job.jobId);
  return <JobCard job={job} tags={tags} viewTo={`/applicant/jobs/${encodeURIComponent(job.jobId)}`}
    secondary={<SaveJobButton state={save} title={job.title} />}
    footer={save.error && <Alert severity="error">{save.error}</Alert>} />;
}

// Find jobs (redesign doc Section 5): search bar with Title, Category, Location and Skills,
// the chosen skills as chips under it, then job cards with Save, soonest deadline first.
export function SearchJobsPage() {
  const [filters, setFilters] = useState<SearchFilters>(emptyFilters);
  const [draft, setDraft] = useState<SearchFilters>(emptyFilters);
  const [token, setToken] = useState('');
  const resource = useLoad(() => searchJobs(token, filters), [token, filters], messages);
  const refs = useLoad(alertReferences, [], messages);
  const invalid = length(draft.query) > 150 ? 'Search title must contain at most 150 characters.'
    : length(draft.location) > 100 ? 'Location must contain at most 100 characters.' : '';
  const apply = (next: SearchFilters) => { setToken(''); setFilters({ ...next, query: next.query.trim(), location: next.location.trim() }); };
  const setTags = (tagIds: string[]) => setDraft(current => ({ ...current, tagIds }));
  function clear() { setDraft(emptyFilters); apply(emptyFilters); }
  const tagName = (id: string) => refs.data?.tags.find(t => t.tagId === id)?.name;
  const category = refs.data?.categories.find(c => c.categoryId === filters.categoryId)?.name;
  const filtered = !!(filters.query || filters.categoryId || filters.location || filters.tagIds.length);
  const jobs = resource.data?.jobs ?? [];
  const total = Number(resource.data?.pagination?.totalCount ?? jobs.length);
  return <Stack spacing={4}>
    <PageHeader title="Find jobs" description="Open jobs and internships, soonest deadline first." />
    <Stack spacing={2}>
      <SearchBar label="Search jobs" submitDisabled={!!invalid} onSubmit={() => { if (!invalid) apply(draft); }}>
        <SearchField label="Job title" value={draft.query} placeholder="Any title" grow={2} onChange={query => setDraft({ ...draft, query })} />
        <SearchSelect label="Category" value={draft.categoryId} disabled={!refs.data}
          options={refs.data?.categories.map(c => [c.categoryId, c.name]) ?? []} onChange={categoryId => setDraft({ ...draft, categoryId })} />
        <SearchField label="Location" value={draft.location} placeholder="Anywhere" suggestions={refs.data?.locations.map(l => l.name)}
          onChange={location => setDraft({ ...draft, location })} />
        {/* Skills are the category-tag tags; like the other fields they apply on Search. */}
        <SearchMultiSelect label="Skills" disabled={!refs.data} options={refs.data?.tags ?? []} optionLabel={t => t.name} optionKey={t => t.tagId}
          value={refs.data?.tags.filter(t => draft.tagIds.includes(t.tagId)) ?? []} onChange={tags => setTags(tags.map(t => t.tagId))} />
      </SearchBar>
      {invalid && <Alert severity="info">{invalid}</Alert>}
      {refs.error && <ErrorState message={refs.error} onRetry={refs.refresh} />}
      {!!draft.tagIds.length && <Stack direction="row" component="ul" aria-label="Selected skills" sx={{ listStyle: 'none', m: 0, p: 0, gap: 1, flexWrap: 'wrap' }}>
        {draft.tagIds.map(id => { const name = tagName(id) ?? 'Skill'; return <Box component="li" key={id}>
          <Chip label={name} onDelete={() => setTags(draft.tagIds.filter(t => t !== id))} aria-label={`${name}, press Delete to remove`}
            sx={{ borderRadius: '8px', fontSize: '0.8125rem', fontWeight: 600, bgcolor: tokens.chipBg, color: tokens.chipText,
              '& .MuiChip-deleteIcon': { color: tokens.chipText, '&:hover': { color: tokens.text } } }} />
        </Box>; })}
      </Stack>}
    </Stack>
    <Stack component="section" aria-label="Results" spacing={2}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <ResultsHeader title={resource.data ? `${total.toLocaleString()} open ${total === 1 ? 'role' : 'roles'}${category ? ` in ${category}` : ''}` : 'Open roles'}
            note="Soonest deadline first" />
        </Box>
        {filtered && <Button onClick={clear}>Clear filters</Button>}
      </Stack>
      {resource.loading && <LoadingRows rows={3} height={140} label="Loading jobs" />}
      {resource.error && <ErrorState message={resource.error} onRetry={resource.refresh} />}
      {resource.data && !jobs.length && <EmptyState title="No open roles match" description="Try other words or fewer filters."
        action={filtered ? <Button variant="outlined" onClick={clear}>Clear filters</Button> : undefined} />}
      {!!jobs.length && <CardGrid label="Jobs">
        {jobs.map(job => <ResultCard key={job.jobId} job={job} tags={(job.tagIds ?? []).map(tagName).filter((n): n is string => !!n)} />)}
      </CardGrid>}
      {resource.data && <Pager token={token} nextToken={resource.data.pagination?.nextPageToken} totalCount={resource.data.pagination?.totalCount}
        count={jobs.length} busy={resource.loading} onPage={setToken} />}
    </Stack>
  </Stack>;
}
