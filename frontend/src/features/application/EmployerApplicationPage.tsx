import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Alert, Box, Button, FormControl, FormControlLabel, FormLabel, Link, MenuItem, Radio, RadioGroup, Stack, Typography } from '@mui/material';
import { Link as RouterLink, useParams } from 'react-router';
import { ApiError } from '../../api/client';
import { Card } from '../../components/Card';
import { LogoSquare } from '../../components/CompanyAvatar';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DeadlineTag } from '../../components/DeadlineTag';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { shortDate } from '../../components/format';
import { ListPanel, ListRow } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { PageHeader } from '../../components/PageHeader';
import { Pager } from '../../components/Pager';
import { StatusChip, ToneChip } from '../../components/StatusChip';
import { isFinalStatus, StatusRail } from '../../components/StatusRail';
import { TextInput } from '../../components/TextInput';
import { jobName, useJob } from '../../components/useJob';
import { tokens } from '../../theme';
import { getApplicantProfileForApplication } from '../applicant-profile/api';
import { profileValues, ProfileView } from '../applicant-profile/ProfileView';
import { useApplicantPhoto } from '../applicant-profile/useApplicantPhoto';
import { listHistory, type HistoryEntry } from '../application-status/api';
import { readMessages } from '../application-status/messages';
import { formatTime, gatewayMessage, grpcCode, isNotFound, length } from '../application-status/shared';
import { StatusHistory } from '../application-status/StatusHistory';
import { useLoad } from '../application-status/useLoad';
import { useAuth } from '../auth/AuthProvider';
import { listInterviews, scheduleInterview } from '../interview/api';
import { listMessages as interviewMessages, scheduleMessages } from '../interview/messages';
import { checkSchedule, maxDetails } from '../interview/schedule';
import { Conversation } from '../messaging/Conversation';
import { getOfferByApplication, makeOffer, type Offer } from '../offer/api';
import { downloadApplicationResume } from '../resume/api';
import { usePdfOpener } from '../resume/usePdfOpener';
import { completeReview, getReview, type Review, type ReviewDecision } from '../review/api';
import { getApplication, type Application } from './api';
import { allPages, statusOf } from './applicant';
import { applicantLabel } from './employer';
import { validId } from './shared';

const back = { to: '/employer/applicants', label: 'Applicants' };
const applicationMessages = {
  [grpcCode.notFound]: 'This application was not found. Open it again from Applicants.',
  [grpcCode.permissionDenied]: 'Only the employer of this job and the applicant can see this application.',
};
const profileMessages = { [grpcCode.permissionDenied]: 'Only the employer of this job can see the applicant\'s profile.' };
const resumeMessages = { [grpcCode.permissionDenied]: 'Only the employer of this job can open the applicant\'s resume.' };
const reviewReadMessages = { [grpcCode.permissionDenied]: 'Only the employer who posted this job can see its review.' };
const reviewMessages = {
  [grpcCode.invalidArgument]: 'Choose a score from 1 to 5 and a decision.',
  [grpcCode.alreadyExists]: 'This application has already been reviewed. The page now shows the review.',
  [grpcCode.permissionDenied]: 'Only the employer who posted this job can review the application.',
  [grpcCode.notFound]: 'This application is not available for review yet. It appears shortly after it is submitted; try again in a moment.',
};
const offerReadMessages = { [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see its offer.' };
const offerMessages = {
  [grpcCode.invalidArgument]: 'Offer terms must contain 1 to 2000 characters.',
  [grpcCode.alreadyExists]: 'An offer already exists for this application. Only one lifetime offer is allowed.',
  [grpcCode.permissionDenied]: 'Only the employer who posted this job can make an offer.',
  [grpcCode.notFound]: 'This application is not available for offers yet. It appears shortly after it is submitted; try again in a moment.',
};
const maxTerms = 2000;
// Status events arrive a moment after an action, so the page reads again shortly after.
const settleMs = 2500;
// NOT_FOUND means "none" for these reads (no review, no offer, no profile), never an error.
const orNull = <T,>(read: Promise<T>) => read.catch((error: unknown) => { if (isNotFound(error)) return null; throw error; });

// Keeps the last loaded value while the same read runs again, so sections and actions do
// not flash back to loading on every refresh.
function useLatest<T>(value: T | undefined) {
  const last = useRef(value);
  if (value !== undefined) last.current = value;
  return last.current;
}

export function EmployerApplicationPage() {
  const { applicationId = '' } = useParams();
  // An address that is not an application ID never reaches the gateway.
  if (!validId(applicationId)) return <Stack spacing={4}>
    <PageHeader back={back} title="Application not found" />
    <EmptyState title="This application does not exist" description="The link may be incomplete. Open the application from Applicants."
      action={<Button component={RouterLink} to="/employer/applicants" variant="outlined">Applicants</Button>} />
  </Stack>;
  return <ApplicationLoader key={applicationId.toLowerCase()} applicationId={applicationId.toLowerCase()} />;
}

function ApplicationLoader({ applicationId }: { applicationId: string }) {
  const resource = useLoad(() => getApplication(applicationId).then(r => r.application), [applicationId], applicationMessages);
  if (resource.loading) return <Stack spacing={4}><PageHeader back={back} title="Loading application…" /><LoadingRows rows={2} height={160} label="Loading application" /></Stack>;
  if (!resource.data) return <Stack spacing={4}><PageHeader back={back} title="Application" />
    <ErrorState message={resource.error || applicationMessages[grpcCode.notFound]} onRetry={resource.refresh} /></Stack>;
  return <ApplicationView app={resource.data} />;
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

// One application for its owning employer (redesign doc Sections 5 and 8, JT-111): the
// applicant (JT-114 name, photo, profile and resume), the job, status and history,
// review, interviews, offer and the conversation. Actions follow the Section 8 table and
// every read is repeated after an action, because events update the status a moment later.
function ApplicationView({ app }: { app: Application }) {
  const { session } = useAuth();
  const id = app.applicationId;
  const [tick, setTick] = useState(0);
  const timer = useRef<number>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  function refreshSoon() {
    setTick(n => n + 1);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setTick(n => n + 1), settleMs);
  }
  const status = useLoad(() => statusOf(app), [id, app.withdrawn, tick], readMessages);
  const history = useLoad(() => allPages(token => listHistory(id, token).then(r => ({ items: r.entries, pagination: r.pagination }))), [id, tick], readMessages);
  const review = useLoad(() => orNull(getReview(id).then(r => r.review)), [id, tick], reviewReadMessages);
  const offer = useLoad(() => orNull(getOfferByApplication(id).then(r => r.offer)), [id, tick], offerReadMessages);
  const current = useLatest(status.data);
  const entries = useLatest(history.data);
  const loadedReview = useLatest(review.data);
  const loadedOffer = useLatest(offer.data);
  // A review or offer saved here is shown at once, before the next read returns it.
  const [savedReview, setSavedReview] = useState<Review>();
  const [savedOffer, setSavedOffer] = useState<Offer>();
  const reviewNow = savedReview ?? loadedReview;
  const offerNow = savedOffer ?? loadedOffer;
  const job = useJob(app.jobId);
  const photo = useApplicantPhoto(id);
  const name = applicantLabel(app);
  // One banner for the latest action; a new action replaces it.
  const [notice, setNotice] = useState('');
  const [dialog, setDialog] = useState<'review' | 'interview' | 'offer' | ''>('');
  const [interviewsVersion, setInterviewsVersion] = useState(0);
  const [messagesTick, setMessagesTick] = useState(0);

  const final = current ? isFinalStatus(current) : false;
  // Section 8: Review only when not reviewed, at Submitted or Under review; Schedule interview
  // at Shortlisted or Interview; Make offer at Interview when there is no offer.
  // A failed read is never taken as "none": no empty text and no action until it reads again.
  const noReview = reviewNow === null && !review.error;
  const noOffer = offerNow === null && !offer.error;
  const canReview = noReview && (current === 'APPLICATION_STATUS_SUBMITTED' || current === 'APPLICATION_STATUS_UNDER_REVIEW');
  const canSchedule = current === 'APPLICATION_STATUS_SHORTLISTED' || current === 'APPLICATION_STATUS_INTERVIEW';
  const canOffer = current === 'APPLICATION_STATUS_INTERVIEW' && noOffer;
  const applied = shortDate(app.timestamps?.createdAt);
  function done(message: string) { setDialog(''); setNotice(message); refreshSoon(); }

  return <Stack spacing={4}>
    <PageHeader back={back} title={name} description={[jobName(job), applied && `applied ${applied}`].filter(Boolean).join(', ')} />
    {notice && <Alert severity="success" role="status">{notice}</Alert>}

    <ApplicantSection applicationId={id} name={name} photo={photo} />

    <Card component="section" label="Job" sx={{ p: { xs: 2.5, sm: 3 } }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1.5 }}>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="h3" component="h2">{jobName(job)}</Typography>
          <Typography variant="body2" color="text.secondary">{job.job?.location || 'Location not given'}</Typography>
        </Box>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
          {job.job && (job.job.status === 'JOB_STATUS_PUBLISHED' ? <DeadlineTag deadline={job.job.deadline} /> : <StatusChip kind="job" status={job.job.status} />)}
          {job.job && <Button component={RouterLink} to={`/employer/jobs/${encodeURIComponent(app.jobId)}`} variant="outlined">View job</Button>}
        </Stack>
      </Stack>
    </Card>

    <Section title="Status" action={<Button disabled={status.loading || history.loading} onClick={refreshSoon}>Refresh</Button>}>
      {status.loading && !current && <LoadingRows rows={1} label="Loading status" />}
      {status.error && <ErrorState message={status.error} onRetry={refreshSoon} />}
      {current && <StatusRail status={current} reachedStatus={reachedBefore(entries ?? [])} />}
      <Box>
        <Typography variant="h3" component="h3" sx={{ mb: 1.5 }}>History</Typography>
        {history.loading && !entries && <LoadingRows rows={2} label="Loading history" />}
        {history.error && !status.error && <ErrorState message={history.error} onRetry={refreshSoon} />}
        {entries && !entries.length && <Typography color="text.secondary">No changes yet. The first entry appears shortly after the application is submitted.</Typography>}
        {!!entries?.length && <StatusHistory entries={entries} />}
      </Box>
    </Section>

    <Section title="Review" action={canReview && <Button variant="contained" onClick={() => { setNotice(''); setDialog('review'); }}>Review application</Button>}>
      {review.loading && reviewNow === undefined && <LoadingRows rows={1} label="Loading review" />}
      {review.error && <ErrorState message={review.error} onRetry={review.refresh} />}
      {reviewNow && <Stack spacing={1}>
        <Box><ToneChip label={reviewNow.decision === 'REVIEW_DECISION_SHORTLISTED' ? 'Shortlisted' : reviewNow.decision === 'REVIEW_DECISION_REJECTED' ? 'Rejected' : 'Unknown'}
          tone={reviewNow.decision === 'REVIEW_DECISION_SHORTLISTED' ? 'accent' : reviewNow.decision === 'REVIEW_DECISION_REJECTED' ? 'danger' : 'neutral'} /></Box>
        <Typography>Score {reviewNow.score} of 5</Typography>
        <Typography variant="body2" color="text.secondary">Reviewed {formatTime(reviewNow.timestamps?.createdAt)}. The applicant sees the outcome through the status, not the score.</Typography>
      </Stack>}
      {noReview && <Typography color="text.secondary">{canReview
        ? 'Not reviewed yet. Each application is reviewed once, and the review cannot be changed.'
        : 'No review was recorded for this application.'}</Typography>}
    </Section>

    <InterviewsSection applicationId={id} version={interviewsVersion}
      action={canSchedule && <Button variant="contained" onClick={() => { setNotice(''); setDialog('interview'); }}>
        {current === 'APPLICATION_STATUS_INTERVIEW' ? 'Schedule another interview' : 'Schedule interview'}</Button>} />

    <Section title="Offer" action={offerNow ? <StatusChip kind="offer" status={offerNow.status} />
      : canOffer && <Button variant="contained" onClick={() => { setNotice(''); setDialog('offer'); }}>Make offer</Button>}>
      {offer.loading && offerNow === undefined && <LoadingRows rows={1} label="Loading offer" />}
      {offer.error && <ErrorState message={offer.error} onRetry={offer.refresh} />}
      {noOffer && <Typography color="text.secondary">{canOffer
        ? 'No offer yet. One offer can be made per application, and it cannot be replaced.'
        : 'No offer yet. An offer can be made once the application reaches the interview stage.'}</Typography>}
      {offerNow && <Box sx={{ p: 2.5, borderRadius: '14px', bgcolor: 'rgba(14,26,59,0.35)', border: `1px solid ${tokens.border}` }}>
        <Typography variant="h3" component="h3" sx={{ mb: 1 }}>Terms</Typography>
        <Typography sx={{ whiteSpace: 'pre-wrap' }}>{offerNow.terms}</Typography>
      </Box>}
    </Section>

    <Section title="Conversation" action={<Button onClick={() => setMessagesTick(n => n + 1)}>Refresh messages</Button>}>
      {session && <Conversation applicationId={id} selfId={session.user.userId} otherName={name} readOnly={final} refreshSignal={messagesTick}
        otherAvatar={<LogoSquare src={photo} name={name} size={32} />} />}
    </Section>

    {dialog === 'review' && <ReviewDialog applicationId={id} name={name} onClose={() => setDialog('')}
      onExists={refreshSoon}
      onDone={saved => { setSavedReview(saved); done(saved.decision === 'REVIEW_DECISION_REJECTED' ? 'Review saved. The application is rejected.' : 'Review saved. The applicant is shortlisted.'); }} />}
    {dialog === 'interview' && <InterviewDialog applicationId={id} onClose={() => setDialog('')}
      onDone={when => { setInterviewsVersion(n => n + 1); done(`Interview scheduled for ${formatTime(when)}.`); }} />}
    {dialog === 'offer' && <OfferDialog applicationId={id} onClose={() => setDialog('')}
      onDone={made => { setSavedOffer(made); done('Offer sent. The applicant can accept or decline it.'); }} />}
  </Stack>;
}

function ApplicantSection({ applicationId, name, photo }: { applicationId: string; name: string; photo: string | null }) {
  const profile = useLoad(() => orNull(getApplicantProfileForApplication(applicationId).then(r => r.profile)), [applicationId], profileMessages);
  const resume = usePdfOpener(resumeMessages, { notFoundIsEmpty: true });
  return <Section title="Applicant">
    <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', rowGap: 1.5 }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', minWidth: 0 }}>
        <LogoSquare src={photo} name={name} size={72} />
        <Typography variant="h3" component="p" sx={{ minWidth: 0 }}>{name}</Typography>
      </Stack>
      <Button variant="outlined" disabled={resume.busy} onClick={() => void resume.open(() => downloadApplicationResume(applicationId), `${name} resume`)}>
        {resume.busy ? 'Opening…' : 'Download resume'}</Button>
    </Stack>
    {resume.missing && <Typography color="text.secondary" role="status">No resume is available for this application.</Typography>}
    {resume.fallback && <Alert severity="info">Your browser blocked the new tab. <Link href={resume.fallback.url} download={resume.fallback.fileName}>Download PDF</Link></Alert>}
    {resume.error && <Alert severity="error">{resume.error}</Alert>}
    {profile.loading && <LoadingRows rows={2} label="Loading profile" />}
    {profile.error && <ErrorState message={profile.error} onRetry={profile.refresh} />}
    {profile.data === null && <Typography color="text.secondary">No profile is available for this application.</Typography>}
    {profile.data && <ProfileView saved={profileValues(profile.data)} heading="h3" />}
  </Section>;
}

function InterviewsSection({ applicationId, version, action }: { applicationId: string; version: number; action: ReactNode }) {
  const [page, setPage] = useState({ version, token: '' });
  // A new interview returns the list to its first page.
  if (page.version !== version) setPage({ version, token: '' });
  const token = page.version === version ? page.token : '';
  const list = useLoad(() => listInterviews(applicationId, token), [applicationId, token, version], interviewMessages);
  const items = list.data?.interviews ?? [];
  return <Section title="Interviews" action={action}>
    {list.loading && <LoadingRows rows={1} label="Loading interviews" />}
    {list.error && <ErrorState message={list.error} onRetry={list.refresh} />}
    {list.data && !items.length && <Typography color="text.secondary">No interviews yet.</Typography>}
    {!!items.length && <ListPanel label="Interviews">
      {items.map(i => <ListRow key={i.interviewId}>
        <Typography sx={{ fontWeight: 700 }}>{formatTime(i.scheduledAt)}</Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5, whiteSpace: 'pre-wrap' }}>{i.details || 'No details given.'}</Typography>
      </ListRow>)}
    </ListPanel>}
    {list.data && <Pager token={token} nextToken={list.data.pagination?.nextPageToken} totalCount={list.data.pagination?.totalCount}
      count={items.length} busy={list.loading} onPage={next => setPage({ version, token: next })} />}
  </Section>;
}

function ReviewDialog({ applicationId, name, onClose, onExists, onDone }: {
  applicationId: string; name: string; onClose: () => void; onExists: () => void; onDone: (review: Review) => void;
}) {
  const [score, setScore] = useState('');
  const [decision, setDecision] = useState<ReviewDecision | ''>('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function submit() {
    if (busy || !score || !decision) return;
    setBusy(true); setError('');
    try { onDone((await completeReview(applicationId, Number(score), decision)).review); }
    catch (reason) {
      setBusy(false);
      setError(gatewayMessage(reason, reviewMessages));
      // Reviewed meanwhile (for example in another tab): the page reads the stored review again.
      if (reason instanceof ApiError && reason.code === grpcCode.alreadyExists) onExists();
    }
  }
  return <ConfirmDialog open title={`Review ${name}`} busy={busy} error={error} destructive={decision === 'REVIEW_DECISION_REJECTED'}
    confirmLabel={decision === 'REVIEW_DECISION_REJECTED' ? 'Reject application' : decision === 'REVIEW_DECISION_SHORTLISTED' ? 'Shortlist applicant' : 'Save review'}
    confirmDisabled={!score || !decision} onCancel={onClose} onConfirm={() => void submit()}>
    <Typography color="text.secondary">A review cannot be changed afterwards. The score is your own assessment and does not decide shortlisting.</Typography>
    <TextInput label="Score" select required value={score} disabled={busy} onChange={e => setScore(e.target.value)}>
      <MenuItem value="" disabled>Choose a score</MenuItem>
      {[1, 2, 3, 4, 5].map(n => <MenuItem key={n} value={String(n)}>{n} of 5</MenuItem>)}
    </TextInput>
    <FormControl required disabled={busy}>
      <FormLabel id="review-decision" sx={{ color: tokens.text, fontSize: '0.875rem', fontWeight: 600, '&.Mui-focused': { color: tokens.text } }}>Decision</FormLabel>
      <RadioGroup aria-labelledby="review-decision" name="decision" value={decision} onChange={e => setDecision(e.target.value as ReviewDecision)}>
        <FormControlLabel value="REVIEW_DECISION_SHORTLISTED" control={<Radio />} label="Shortlist" />
        <FormControlLabel value="REVIEW_DECISION_REJECTED" control={<Radio />} label="Reject" />
      </RadioGroup>
    </FormControl>
  </ConfirmDialog>;
}

function InterviewDialog({ applicationId, onClose, onDone }: { applicationId: string; onClose: () => void; onDone: (scheduledAt?: string) => void }) {
  const [when, setWhen] = useState('');
  const [details, setDetails] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const checked = checkSchedule(when, details);
  async function submit() {
    if (busy) return;
    // Checked again on send, because "now" has moved on since the time was chosen.
    const now = checkSchedule(when, details);
    if (!now.ok) { setError(now.message); return; }
    setBusy(true); setError('');
    try { onDone((await scheduleInterview(applicationId, now.scheduledAt, details.trim())).interview.scheduledAt); }
    catch (reason) { setError(gatewayMessage(reason, scheduleMessages)); setBusy(false); }
  }
  return <ConfirmDialog open title="Schedule an interview" busy={busy} error={error} confirmLabel="Schedule interview" confirmDisabled={!checked.ok}
    onCancel={onClose} onConfirm={() => void submit()}>
    <TextInput label="Date and time" type="datetime-local" required value={when} disabled={busy}
      helperText="Your local time. Must be in the future and within 365 days." onChange={e => setWhen(e.target.value)} />
    <TextInput label="Details" multiline minRows={3} value={details} disabled={busy}
      helperText={`Optional, for example the place or meeting link. ${length(details)} of ${maxDetails} characters.`} onChange={e => setDetails(e.target.value)} />
    {(when || details) && !checked.ok && <Alert severity="warning">{checked.message}</Alert>}
  </ConfirmDialog>;
}

function OfferDialog({ applicationId, onClose, onDone }: { applicationId: string; onClose: () => void; onDone: (offer: Offer) => void }) {
  const [terms, setTerms] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const count = length(terms);
  async function submit() {
    if (busy || count < 1 || count > maxTerms) return;
    setBusy(true); setError('');
    try { onDone((await makeOffer(applicationId, terms.trim())).offer); }
    catch (reason) { setError(gatewayMessage(reason, offerMessages)); setBusy(false); }
  }
  return <ConfirmDialog open title="Make an offer" busy={busy} error={error} confirmLabel="Send offer" confirmDisabled={count < 1 || count > maxTerms}
    onCancel={onClose} onConfirm={() => void submit()}>
    <Typography color="text.secondary">Each application can have one offer, and it cannot be replaced. The applicant accepts or declines it.</Typography>
    <TextInput label="Offer terms" required multiline minRows={5} value={terms} disabled={busy} error={count > maxTerms}
      helperText={`${count} of ${maxTerms} characters`} onChange={e => setTerms(e.target.value)} />
    {count > maxTerms && <Alert severity="warning">Offer terms can have at most {maxTerms} characters.</Alert>}
  </ConfirmDialog>;
}
