import { useState } from 'react';
import { Alert, Button, Card, CardContent, Dialog, DialogActions, DialogContent, DialogTitle, FormControl, FormControlLabel, FormLabel, MenuItem, Radio, RadioGroup, Stack, TextField, Typography } from '@mui/material';
import { Link, useParams } from 'react-router';
import { ResourceState, validId } from '../application/shared';
import { formatTime, gatewayMessage, grpcCode, isNotFound } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { completeReview, getReview, type Review, type ReviewDecision } from './api';

const decisionLabel = (d: Review['decision']) => d === 'REVIEW_DECISION_SHORTLISTED' ? 'Shortlisted' : d === 'REVIEW_DECISION_REJECTED' ? 'Rejected' : 'Unknown';
const messages = {
  [grpcCode.invalidArgument]: 'Choose a score from 1 to 5 and a decision.',
  [grpcCode.alreadyExists]: 'This application has already been reviewed. Refresh to see the review.',
  [grpcCode.permissionDenied]: 'Only the employer who posted this job can review the application.',
  [grpcCode.notFound]: 'This application is not available for review yet. It appears shortly after it is submitted; try again in a moment.',
};

export function ReviewPage() {
  const { applicationId = '' } = useParams();
  if (!validId(applicationId)) return <Alert severity="error">The application ID in the address is not valid.</Alert>;
  return <ReviewView applicationId={applicationId.toLowerCase()} />;
}

function ReviewView({ applicationId }: { applicationId: string }) {
  const existing = useLoad(() => getReview(applicationId), [applicationId], messages);
  const [saved, setSaved] = useState<Review>();
  const [score, setScore] = useState('');
  const [decision, setDecision] = useState<ReviewDecision | ''>('');
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const review = saved ?? existing.data?.review;
  const notReviewed = !review && !existing.loading && isNotFound(existing.failure);
  async function submit() {
    if (busy || !score || !decision) return;
    setBusy(true); setError('');
    try { setSaved((await completeReview(applicationId, Number(score), decision)).review); setConfirming(false); }
    catch (reason) { setError(gatewayMessage(reason, messages)); setConfirming(false); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Review application</Typography>
    <Typography variant="body2">Application: {applicationId}</Typography>
    <Button component={Link} to={`/employer/applications/${applicationId}/status`}>View status</Button>
    {existing.loading && <ResourceState loading error="" />}
    {!existing.loading && existing.error && !notReviewed && <><Alert severity="error">{existing.error}</Alert><Button onClick={existing.refresh}>Try again</Button></>}
    {review && <Card><CardContent><Stack spacing={1}>
      <Typography variant="h6" component="h2">Review completed</Typography>
      <Typography>Decision: {decisionLabel(review.decision)}</Typography>
      <Typography>Score: {review.score} of 5</Typography>
      <Typography>Reviewed: {formatTime(review.timestamps?.createdAt)}</Typography>
      <Typography variant="body2">The applicant sees the outcome through the application status, not the score.</Typography>
    </Stack></CardContent></Card>}
    {notReviewed && <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); if (score && decision) setConfirming(true); }}>
      <Typography>Each application is reviewed once. The score is your own assessment and does not decide shortlisting.</Typography>
      <TextField select label="Score" required value={score} disabled={busy} onChange={e => setScore(e.target.value)}>
        {[1, 2, 3, 4, 5].map(n => <MenuItem key={n} value={String(n)}>{n}</MenuItem>)}
      </TextField>
      <FormControl required disabled={busy}><FormLabel id="decision-label">Decision</FormLabel>
        <RadioGroup aria-labelledby="decision-label" name="decision" value={decision} onChange={e => setDecision(e.target.value as ReviewDecision)}>
          <FormControlLabel value="REVIEW_DECISION_SHORTLISTED" control={<Radio />} label="Shortlist" />
          <FormControlLabel value="REVIEW_DECISION_REJECTED" control={<Radio />} label="Reject" />
        </RadioGroup>
      </FormControl>
      {error && <Alert severity="error">{error}</Alert>}
      <Button type="submit" variant="contained" disabled={busy || !score || !decision}>Complete review</Button>
    </Stack>}
    <Dialog open={confirming} onClose={() => { if (!busy) setConfirming(false); }}>
      <DialogTitle>{decision === 'REVIEW_DECISION_SHORTLISTED' ? 'Shortlist' : 'Reject'} this application?</DialogTitle>
      <DialogContent>A review cannot be changed afterwards. Score: {score} of 5.</DialogContent>
      <DialogActions><Button disabled={busy} onClick={() => setConfirming(false)}>Cancel</Button><Button disabled={busy} onClick={() => void submit()}>Confirm review</Button></DialogActions>
    </Dialog>
  </Stack>;
}
