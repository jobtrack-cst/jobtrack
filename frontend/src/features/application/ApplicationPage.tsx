import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { Link, useParams } from 'react-router';
import { Card } from '../../components/Card';
import { LogoSquare, useCompany } from '../../components/CompanyAvatar';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DeadlineTag } from '../../components/DeadlineTag';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { ListPanel, ListRow } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { StatusChip } from '../../components/StatusChip';
import { isFinalStatus, StatusRail } from '../../components/StatusRail';
import { jobName, useJob } from '../../components/useJob';
import { tokens } from '../../theme';
import { listHistory, type HistoryEntry } from '../application-status/api';
import { formatTime, grpcCode, isNotFound, statusLabel } from '../application-status/shared';
import { readMessages } from '../application-status/messages';
import { useLoad } from '../application-status/useLoad';
import { useAuth } from '../auth/AuthProvider';
import { listInterviews } from '../interview/api';
import { listMessages as interviewMessages } from '../interview/messages';
import { Conversation } from '../messaging/Conversation';
import { getOfferByApplication, respondOffer, type Offer } from '../offer/api';
import { getApplication, withdrawApplication, type Application } from './api';
import { allPages, statusOf } from './applicant';
import { errorMessage, validId } from './shared';

const back = { to: '/applicant/applications', label: 'Applications' };
const applicationMessages = {
  [grpcCode.notFound]: 'This application was not found. Open it again from Applications.',
  [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see it.',
};
// Status events arrive a moment after an action, so the page reads again shortly after.
const settleMs = 2500;

export function ApplicationPage() {
  const { applicationId = '' } = useParams();
  // An address that is not an application ID never reaches the gateway.
  if (!validId(applicationId)) return <Stack spacing={4}>
    <PageHeader back={back} title="Application not found" />
    <EmptyState title="This application does not exist" description="The link may be incomplete. Open the application from Applications."
      action={<Button component={Link} to="/applicant/applications" variant="outlined">Applications</Button>} />
  </Stack>;
  return <ApplicationLoader key={applicationId.toLowerCase()} applicationId={applicationId.toLowerCase()} />;
}

function ApplicationLoader({ applicationId }: { applicationId: string }) {
  const resource = useLoad(() => getApplication(applicationId).then(r => r.application), [applicationId], applicationMessages);
  if (resource.loading) return <Stack spacing={4}><PageHeader back={back} title="Loading application…" /><LoadingRows rows={2} height={160} label="Loading application" /></Stack>;
  if (!resource.data) return <Stack spacing={4}><PageHeader back={back} title="Application" />
    <ErrorState message={resource.error || applicationMessages[grpcCode.notFound]} onRetry={resource.refresh} /></Stack>;
  return <ApplicationView initial={resource.data} />;
}

function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return <Card component="section" label={title} sx={{ p: { xs: 2.5, sm: 4 } }}>
    <Stack spacing={2.5}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1 }}>
        <Typography variant="h2" component="h2">{title}</Typography>
        {action}
      </Stack>
      {children}
    </Stack>
  </Card>;
}

// The furthest step reached before a final state, for the rail.
function reachedBefore(entries: HistoryEntry[]) {
  return [...entries].reverse().map(e => e.newStatus).find(s => s && !isFinalStatus(s) && s !== 'APPLICATION_STATUS_UNSPECIFIED');
}

function ApplicationView({ initial }: { initial: Application }) {
  const { session } = useAuth();
  const [app, setApp] = useState(initial);
  const id = app.applicationId;
  const [tick, setTick] = useState(0);
  const timer = useRef<number>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  // Read status, history and offer again now and once more after the events settle.
  function refreshSoon() {
    setTick(n => n + 1);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setTick(n => n + 1), settleMs);
  }
  const status = useLoad(() => statusOf(app), [id, app.withdrawn, tick], readMessages);
  const history = useLoad(() => allPages(token => listHistory(id, token).then(r => ({ items: r.entries, pagination: r.pagination }))), [id, tick], readMessages);
  const offer = useLoad(() => getOfferByApplication(id).then(r => r.offer, (e: unknown) => { if (isNotFound(e)) return null; throw e; }), [id, tick], {});
  const job = useJob(app.jobId);
  const { company } = useCompany(app.employerId);
  const [withdrawing, setWithdrawing] = useState(false);
  const [messagesTick, setMessagesTick] = useState(0);
  const [notice, setNotice] = useState('');
  const current = status.data;
  const final = current ? isFinalStatus(current) : false;
  const companyName = company?.name || 'Employer';
  const meta = [company?.name, job.job?.location].filter(Boolean).join(', ');
  const applied = shortDate(app.timestamps?.createdAt);
  return <Stack spacing={4}>
    <PageHeader back={back} title={jobName(job)} description={[meta, applied && `Applied ${applied}`].filter(Boolean).join('. ') || undefined}
      action={<Button component={Link} to={`/applicant/jobs/${encodeURIComponent(app.jobId)}`} variant="outlined">View job</Button>} />
    {notice && <Alert severity="success" role="status">{notice}</Alert>}

    <Card component="section" label="Job" sx={{ p: { xs: 2.5, sm: 3 } }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1.5 }}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          <LogoSquare src={company?.url} name={company?.name} size={48} />
          <Box sx={{ minWidth: 0 }}>
            <Typography sx={{ fontWeight: 700 }}>{companyName}</Typography>
            <Typography variant="body2" color="text.secondary">{job.job?.location || 'Location not given'}</Typography>
          </Box>
        </Stack>
        <Stack direction="row" spacing={1}>
          {job.job && job.job.status !== 'JOB_STATUS_PUBLISHED' && <StatusChip kind="job" status={job.job.status} />}
          {job.job && <DeadlineTag deadline={job.job.deadline} />}
        </Stack>
      </Stack>
      {job.job?.description && <Typography color="text.secondary" sx={{ mt: 2, whiteSpace: 'pre-wrap', display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
        {job.job.description}
      </Typography>}
    </Card>

    <Section title="Status" action={current && !final && <Button color="error" variant="outlined" onClick={() => { setNotice(''); setWithdrawing(true); }}>Withdraw</Button>}>
      {status.loading && !current && <LoadingRows rows={1} label="Loading status" />}
      {status.error && <ErrorState message={status.error} onRetry={refreshSoon} />}
      {current && <StatusRail status={current} reachedStatus={reachedBefore(history.data ?? [])} />}
      <Box>
        <Typography variant="h3" component="h3" sx={{ mb: 1.5 }}>History</Typography>
        {history.loading && !history.data && <LoadingRows rows={2} label="Loading history" />}
        {history.error && !status.error && <ErrorState message={history.error} onRetry={refreshSoon} />}
        {history.data && !history.data.length && <Typography color="text.secondary">No changes yet. The first entry appears shortly after you apply.</Typography>}
        {!!history.data?.length && <ListPanel label="Status history">
          {history.data.map((e, i) => <ListRow key={i} sx={{ py: 1.75 }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={{ xs: 0.25, sm: 2 }} sx={{ justifyContent: 'space-between' }}>
              <Typography><Box component="strong">{statusLabel(e.newStatus)}</Box>
                <Typography component="span" color="text.secondary">{` from ${statusLabel(e.previousStatus)}`}</Typography></Typography>
              <Typography variant="body2" color="text.secondary">{formatTime(e.occurredAt)}</Typography>
            </Stack>
          </ListRow>)}
        </ListPanel>}
      </Box>
    </Section>

    <Interviews applicationId={id} />

    <OfferSection offer={offer.data} loading={offer.loading} error={offer.error} onRetry={offer.refresh} withdrawn={!!app.withdrawn}
      onAnswered={decision => { setNotice(decision === 'accept' ? 'Offer accepted.' : 'Offer declined.'); refreshSoon(); }} />

    <Section title="Conversation" action={<Button onClick={() => setMessagesTick(n => n + 1)}>Refresh messages</Button>}>
      {session && <Conversation applicationId={id} selfId={session.user.userId} otherName={companyName} readOnly={final} refreshSignal={messagesTick}
        otherAvatar={<LogoSquare src={company?.url} name={company?.name} size={32} />} />}
    </Section>

    {withdrawing && <WithdrawDialog applicationId={id} onClose={() => setWithdrawing(false)}
      onDone={next => { setWithdrawing(false); setApp(next ?? { ...app, withdrawn: true }); setNotice('Application withdrawn.'); refreshSoon(); }} />}
  </Stack>;
}

function Interviews({ applicationId }: { applicationId: string }) {
  const [token, setToken] = useState('');
  const list = useLoad(() => listInterviews(applicationId, token), [applicationId, token], interviewMessages);
  const items = list.data?.interviews ?? [];
  return <Section title="Interviews">
    {list.loading && <LoadingRows rows={1} label="Loading interviews" />}
    {list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !items.length && <Typography color="text.secondary">No interviews yet. The employer schedules them, and they appear here.</Typography>}
    {!!items.length && <ListPanel label="Interviews">
      {items.map(i => <ListRow key={i.interviewId}>
        <Typography sx={{ fontWeight: 700 }}>{formatTime(i.scheduledAt)}</Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5, whiteSpace: 'pre-wrap' }}>{i.details || 'No details given.'}</Typography>
      </ListRow>)}
    </ListPanel>}
    {list.data && <Pager token={token} nextToken={list.data.pagination?.nextPageToken} totalCount={list.data.pagination?.totalCount}
      count={items.length} busy={list.loading} onPage={setToken} />}
  </Section>;
}

function OfferSection({ offer, loading, error, onRetry, withdrawn, onAnswered }: {
  offer?: Offer | null; loading: boolean; error: string; onRetry: () => void; withdrawn: boolean; onAnswered: (decision: 'accept' | 'decline') => void;
}) {
  const [decision, setDecision] = useState<'accept' | 'decline'>();
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState('');
  async function answer() {
    if (!offer || !decision || busy) return;
    setBusy(true); setFailure('');
    try { await respondOffer(offer.offerId, decision); setDecision(undefined); onAnswered(decision); }
    catch (reason) { setFailure(errorMessage(reason)); }
    finally { setBusy(false); }
  }
  const pending = offer?.status === 'OFFER_STATUS_PENDING' && !withdrawn;
  return <Section title="Offer" action={offer && <StatusChip kind="offer" status={offer.status} />}>
    {loading && offer === undefined && <LoadingRows rows={1} label="Loading offer" />}
    {error && <ErrorState message={error} onRetry={onRetry} />}
    {offer === null && <Typography color="text.secondary">No offer yet. If the employer makes one, it shows here.</Typography>}
    {offer && <>
      <Box sx={{ p: 2.5, borderRadius: '14px', bgcolor: 'rgba(14,26,59,0.35)', border: `1px solid ${tokens.border}` }}>
        <Typography variant="h3" component="h3" sx={{ mb: 1 }}>Terms</Typography>
        <Typography sx={{ whiteSpace: 'pre-wrap' }}>{offer.terms}</Typography>
      </Box>
      {pending && <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
        <Button variant="contained" color="success" onClick={() => { setFailure(''); setDecision('accept'); }}>Accept offer</Button>
        <Button variant="outlined" color="error" onClick={() => { setFailure(''); setDecision('decline'); }}>Decline offer</Button>
      </Stack>}
    </>}
    <ConfirmDialog open={!!decision} title={decision === 'accept' ? 'Accept this offer?' : 'Decline this offer?'}
      confirmLabel={decision === 'accept' ? 'Accept offer' : 'Decline offer'} destructive={decision === 'decline'} busy={busy} error={failure}
      onCancel={() => setDecision(undefined)} onConfirm={() => void answer()}>
      <Typography>This decision is final and cannot be changed.</Typography>
    </ConfirmDialog>
  </Section>;
}

function WithdrawDialog({ applicationId, onClose, onDone }: { applicationId: string; onClose: () => void; onDone: (app?: Application) => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function withdraw() {
    if (busy) return;
    setBusy(true); setError('');
    try { onDone((await withdrawApplication(applicationId)).application); }
    catch (reason) { setError(errorMessage(reason)); setBusy(false); }
  }
  return <ConfirmDialog open title="Withdraw this application?" confirmLabel="Withdraw" destructive busy={busy} error={error}
    onCancel={onClose} onConfirm={() => void withdraw()}>
    <Typography>The employer will see that you withdrew. You cannot apply for this job again.</Typography>
  </ConfirmDialog>;
}
