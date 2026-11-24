import { useEffect, useRef, useState } from 'react';
import { Box, Button, Stack, Typography, useMediaQuery, useTheme } from '@mui/material';
import { Link, Navigate, useParams, useSearchParams } from 'react-router';
import { Card } from '../../components/Card';
import { LogoSquare, useCompany } from '../../components/CompanyAvatar';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { Icon } from '../../components/Icon';
import { ListPanel, ListRow, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager, pageSize } from '../../components/Pager';
import { SearchBar, SearchSelect } from '../../components/SearchBar';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { isFinalStatus } from '../../components/StatusRail';
import { jobName, rememberJob, useJob } from '../../components/useJob';
import { tokens } from '../../theme';
import { ApplicantAvatar } from '../applicant-profile/ApplicantAvatar';
import { useApplicantPhoto } from '../applicant-profile/useApplicantPhoto';
import { readMessages } from '../application-status/messages';
import { useLoad } from '../application-status/useLoad';
import { statusOf } from '../application/applicant';
import { applicantLabel, loadFilterJobs } from '../application/employer';
import { validId } from '../application/shared';
import { useAuth } from '../auth/AuthProvider';
import type { Message } from './api';
import { Conversation } from './Conversation';
import type { Application } from '../application/api';
import { loadApplications, previewOf, rowTime, sortInbox, withLastMessages, type InboxItem } from './inbox';

// Keeps the last loaded value while the same read runs again, so the list and the open
// thread do not flash back to loading on Refresh.
function useLatest<T>(value: T | undefined) {
  const last = useRef(value);
  if (value !== undefined) last.current = value;
  return last.current;
}

const time = (m?: Message | null) => Date.parse(m?.timestamps?.createdAt ?? '') || 0;
const idOf = (item: InboxItem) => item.app.applicationId.toLowerCase();

// Messages inbox for applicants and employers (redesign doc Section 6, JT-113): one
// conversation per application, newest activity first, with the thread beside the list
// from 900px up and as its own screen on phones. The open conversation is the path
// (/<role>/messages/:applicationId) and the employer's job filter is ?job=, so reloads
// and Back keep both.
export function MessagesPage() {
  const { session } = useAuth();
  const employer = session?.user.role === 'ROLE_EMPLOYER';
  const selfId = session?.user.userId ?? '';
  const base = employer ? '/employer' : '/applicant';
  const { applicationId = '' } = useParams();
  const selectedId = validId(applicationId) ? applicationId.toLowerCase() : '';
  const [params, setParams] = useSearchParams();
  const raw = params.get('job') ?? '';
  const jobId = employer && validId(raw) ? raw.toLowerCase() : '';
  const search = jobId ? `?job=${encodeURIComponent(jobId)}` : '';
  const listPath = `${base}/messages${search}`;
  const theme = useTheme();
  const wide = useMediaQuery(theme.breakpoints.up('md'), { noSsr: true });

  // Two stages: the applications (enough to open a thread), then every row's last message
  // (needed to sort the list). Refresh reads the applications again, which reads the rest.
  const applications = useLoad(loadApplications, [], {});
  const apps = useLatest(applications.data);
  const previews = useLoad(async () => apps ? withLastMessages(apps) : undefined, [apps], {});
  const loaded = useLatest(previews.data);
  const inbox = { loading: applications.loading || previews.loading, error: applications.error, refresh: applications.refresh };
  // Messages sent here since the list was read; a newer one replaces the row's preview.
  const [sent, setSent] = useState<Record<string, Message>>({});
  const [threadTick, setThreadTick] = useState(0);
  const items = loaded ? sortInbox(loaded.map(item => {
    const mine = sent[idOf(item)];
    return mine && time(mine) >= time(item.last) ? { ...item, last: mine, failed: false } : item;
  })) : undefined;

  // The list is paged in the browser, because sorting needs every conversation. A new
  // job filter starts on page 1 (set during render, as on the Applicants page).
  const [page, setPage] = useState({ jobId, offset: 0 });
  if (page.jobId !== jobId) setPage({ jobId, offset: 0 });

  // The job filter, with the same options as the Applicants page. Applicants have none.
  const [choice, setChoice] = useState(jobId);
  useEffect(() => { setChoice(jobId); }, [jobId]);
  const jobs = useLoad(async () => {
    if (!employer) return [];
    const options = await loadFilterJobs();
    options.forEach(o => rememberJob(o.job));
    return options;
  }, [employer], {});
  const options: [string, string][] = (jobs.data ?? []).map(o => [o.job.jobId.toLowerCase(), o.label]);
  if (choice && jobs.data && !options.some(([id]) => id === choice)) options.unshift([choice, 'Job no longer listed']);
  const chosen = options.find(([id]) => id === jobId)?.[1];
  function applyFilter() {
    if (choice === jobId) { setPage({ jobId, offset: 0 }); return; }
    setParams(previous => {
      const next = new URLSearchParams(previous);
      if (choice) next.set('job', choice); else next.delete('job');
      return next;
    }, { replace: true });
  }

  const shown = items && jobId ? items.filter(i => i.app.jobId.toLowerCase() === jobId) : items;
  const count = shown?.length ?? 0;
  const lastPage = Math.max(0, Math.floor((count - 1) / pageSize) * pageSize);
  const offset = Math.min(page.jobId === jobId ? page.offset : 0, lastPage);
  const visible = shown?.slice(offset, offset + pageSize) ?? [];
  const selected = selectedId ? apps?.find(app => app.applicationId.toLowerCase() === selectedId) : undefined;

  if (applicationId && !selectedId) return <Navigate to={listPath} replace />;

  function refresh() { inbox.refresh(); setThreadTick(n => n + 1); }
  const refreshButton = <Button variant="outlined" startIcon={<Icon name="refresh" />} disabled={inbox.loading} onClick={refresh}>
    {inbox.loading && loaded ? 'Refreshing…' : 'Refresh'}</Button>;

  const filter = employer && <>
    <SearchBar label="Filter conversations" submitLabel="Show conversations" onSubmit={applyFilter}>
      <SearchSelect label="Job" anyLabel="All jobs" value={choice} onChange={setChoice} options={options} disabled={jobs.loading} grow={3} />
    </SearchBar>
    {jobs.error && <ErrorState message="Your jobs could not be loaded for the filter. Conversations for all jobs are still shown." onRetry={jobs.refresh} />}
  </>;

  const list = <Stack spacing={2} sx={{ minWidth: 0 }}>
    {inbox.error && <ErrorState message={inbox.error} onRetry={inbox.refresh} />}
    {!items && inbox.loading && <LoadingRows rows={4} label="Loading conversations" />}
    {items && !items.length && <EmptyState title="No conversations yet"
      description={employer ? 'Each application for your jobs gets a conversation with the applicant.' : 'Each job you apply for gets a conversation with its employer.'}
      action={<Button component={Link} to={employer ? '/employer/jobs' : '/applicant/jobs'} variant="outlined">{employer ? 'Your jobs' : 'Find jobs'}</Button>} />}
    {items && !!items.length && !count && <EmptyState title="No conversations for this job" description="Conversations for this job show here as applications arrive." />}
    {!!count && <>
      <ResultsHeader title={`${count.toLocaleString()} ${count === 1 ? 'conversation' : 'conversations'}`}
        note={jobId ? `For ${chosen ?? 'the chosen job'}` : 'Newest first'} />
      <ListPanel label="Conversations">
        {visible.map(item => <Row key={idOf(item)} item={item} employer={employer} selfId={selfId} current={idOf(item) === selectedId}
          to={`${base}/messages/${encodeURIComponent(idOf(item))}${search}`} />)}
      </ListPanel>
      <Pager token={offset ? String(offset) : ''} nextToken={offset + pageSize < count ? String(offset + pageSize) : undefined}
        totalCount={count} count={visible.length} onPage={token => setPage({ jobId, offset: Number(token) || 0 })} />
    </>}
  </Stack>;

  function thread() {
    if (!selectedId) return wide ? <EmptyState title="Choose a conversation" description="Pick a conversation from the list to read and reply." /> : null;
    if (!apps) return applications.loading ? <LoadingRows rows={2} height={160} label="Loading conversation" /> : null;
    if (!selected) return <EmptyState title="This conversation is not available"
      description="It may belong to another account or is no longer listed. Choose one from your conversations."
      action={<Button component={Link} to={listPath} variant="outlined">All messages</Button>} />;
    const props = { app: selected, selfId, refreshSignal: threadTick, onSent: (m: Message) => setSent(s => ({ ...s, [selectedId]: m })) };
    return employer ? <EmployerThread key={selectedId} {...props} /> : <ApplicantThread key={selectedId} {...props} />;
  }

  const description = employer ? 'Your conversations with applicants, one for each application.' : 'Your conversations with employers, one for each application.';
  // Phones: the thread is its own screen, with a link back to the list.
  if (!wide && selectedId) return <Stack spacing={4}>
    <PageHeader back={{ to: listPath, label: 'All messages' }} title="Messages" action={refreshButton} />
    {!apps && inbox.error && <ErrorState message={inbox.error} onRetry={inbox.refresh} />}
    {thread()}
  </Stack>;
  return <Stack spacing={4}>
    <PageHeader title="Messages" description={description} action={refreshButton} />
    {filter}
    {wide
      ? <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(280px, 360px) minmax(0, 1fr)', gap: 3, alignItems: 'start' }}>
        {list}
        <Box sx={{ minWidth: 0 }}>{thread()}</Box>
      </Box>
      : list}
  </Stack>;
}

// One conversation row: the other side's avatar and name, the job, and the last message.
function Row({ item, employer, selfId, current, to }: { item: InboxItem; employer: boolean; selfId: string; current: boolean; to: string }) {
  const { app, last, failed } = item;
  const job = useJob(app.jobId);
  const { company } = useCompany(employer ? undefined : app.employerId);
  const name = employer ? applicantLabel(app) : company?.name || 'Employer';
  const preview = failed ? 'Last message unavailable'
    : last ? `${last.senderId === selfId ? 'You: ' : ''}${previewOf(last.content)}` : 'No messages yet';
  const when = rowTime(last?.timestamps?.createdAt);
  return <ListRow to={to} current={current} sx={{ px: 2, py: 2 }}>
    <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
      {employer ? <ApplicantAvatar applicationId={app.applicationId} name={name} size={44} />
        : <LogoSquare src={company?.url} name={company?.name} size={44} />}
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline', justifyContent: 'space-between', minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 700, minWidth: 0 }}>{name}</Typography>
          {when && <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>{when}</Typography>}
        </Stack>
        <Typography noWrap variant="body2" color="text.secondary">{jobName(job)}</Typography>
        <Typography noWrap variant="body2" sx={{ mt: 0.25, color: last && !failed ? tokens.text : tokens.textSecondary }}>{preview}</Typography>
      </Box>
    </Stack>
  </ListRow>;
}

interface ThreadProps { app: Application; selfId: string; refreshSignal: number; onSent: (message: Message) => void }

function ApplicantThread(props: ThreadProps) {
  const { company } = useCompany(props.app.employerId);
  return <ThreadView {...props} name={company?.name || 'Employer'} avatar={company?.url ?? null}
    applicationPath={`/applicant/applications/${encodeURIComponent(props.app.applicationId)}`} />;
}

// The photo is read once for the header and the bubbles, and revoked on unmount.
function EmployerThread(props: ThreadProps) {
  const photo = useApplicantPhoto(props.app.applicationId);
  return <ThreadView {...props} name={applicantLabel(props.app)} avatar={photo}
    applicationPath={`/employer/applicants/${encodeURIComponent(props.app.applicationId)}`} />;
}

// The open conversation: who it is with, the job and status, then the shared thread and
// composer. Final applications are view only (redesign doc Section 8); while the status
// is loading or failed the composer shows, as on the application pages.
function ThreadView({ app, selfId, refreshSignal, onSent, name, avatar, applicationPath }: ThreadProps & { name: string; avatar: string | null; applicationPath: string }) {
  const status = useLoad(() => statusOf(app), [app.applicationId, app.withdrawn, refreshSignal], readMessages);
  const current = useLatest(status.data);
  const job = useJob(app.jobId);
  return <Card component="section" label={`Conversation with ${name}`} sx={{ p: { xs: 2, sm: 3 } }}>
    <Stack spacing={2.5}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1.5 }}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0, flex: '1 1 200px' }}>
          <LogoSquare src={avatar} name={name} size={48} />
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="h2" component="h2" sx={{ fontSize: '1.25rem' }}>{name}</Typography>
            <Typography variant="body2" color="text.secondary">{jobName(job)}</Typography>
          </Box>
        </Stack>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
          {current ? <StatusChip kind="application" status={current} /> : status.error ? <ToneChip label="Status unavailable" tone="neutral" /> : null}
          <Button component={Link} to={applicationPath} variant="outlined">View application</Button>
        </Stack>
      </Stack>
      <Conversation applicationId={app.applicationId} selfId={selfId} otherName={name} readOnly={current ? isFinalStatus(current) : false}
        otherAvatar={<LogoSquare src={avatar} name={name} size={32} />} refreshSignal={refreshSignal} onSent={onSent} />
    </Stack>
  </Card>;
}
