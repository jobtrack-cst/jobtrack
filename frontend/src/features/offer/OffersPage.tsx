import { useState } from 'react';
import { Alert, Button, Dialog, DialogActions, DialogTitle, Stack, TextField, Typography } from '@mui/material';
import { useParams, useSearchParams } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { errorMessage, validId } from '../application/shared';
import { getOffer, makeOffer, respondOffer, type Offer } from './api';

export function OffersPage({ create = false }: { create?: boolean }) {
  const { applicationId = '' } = useParams();
  const [params] = useSearchParams();
  const applicant = useAuth().session?.user.role === 'ROLE_APPLICANT';
  const [id, setId] = useState(create ? applicationId : params.get('offerId') ?? '');
  const [terms, setTerms] = useState('');
  const [offer, setOffer] = useState<Offer>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [decision, setDecision] = useState<'accept' | 'decline'>();
  async function run(action: () => Promise<{ offer: Offer }>) {
    if (busy) return;
    setBusy(true); setError('');
    try { setOffer((await action()).offer); setDecision(undefined); }
    catch (reason) { setError(errorMessage(reason)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">{create ? 'Make offer' : 'View offer'}</Typography>
    {!create && <Typography>Enter the offer ID shared with you. The current API supports viewing one offer at a time.</Typography>}
    {create && <Typography>One offer per application is allowed. Terms must contain 1 to 2000 characters.</Typography>}
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); void run(() => create ? makeOffer(id.trim(), terms.trim()) : getOffer(id.trim())); }}>
      <TextField label={create ? 'Application ID' : 'Offer ID'} required value={id} disabled={busy} onChange={e => { setId(e.target.value); setOffer(undefined); setError(''); }} />
      {create && <TextField label="Offer terms" required multiline minRows={4} value={terms} disabled={busy || !!offer} onChange={e => setTerms(e.target.value)} helperText={`${Array.from(terms.trim()).length}/2000 characters`} />}
      <Button type="submit" disabled={busy || !validId(id.trim()) || (create && (!!offer || !terms.trim() || Array.from(terms.trim()).length > 2000))}>{create ? 'Make offer' : 'Load offer'}</Button>
    </Stack>
    {error && <Alert severity="error">{error}</Alert>}
    {offer && <><Alert severity="success">{create ? 'Offer made. Share this offer ID with the applicant: ' : 'Offer ID: '}{offer.offerId}</Alert>
      <Typography>Application: {offer.applicationId}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{offer.terms}</Typography><Typography>Status: {offer.status.replace('OFFER_STATUS_', '')}</Typography>
      {applicant && offer.status === 'OFFER_STATUS_PENDING' && <Stack direction="row" spacing={2}>
        <Button disabled={busy} onClick={() => setDecision('accept')}>Accept offer</Button><Button disabled={busy} onClick={() => setDecision('decline')}>Decline offer</Button>
      </Stack>}</>}
    <Dialog open={!!decision} onClose={() => { if (!busy) setDecision(undefined); }}><DialogTitle>{decision === 'accept' ? 'Accept' : 'Decline'} this offer? This decision is final.</DialogTitle><DialogActions>
      <Button disabled={busy} onClick={() => setDecision(undefined)}>Cancel</Button><Button disabled={busy} onClick={() => { if (offer && decision) void run(() => respondOffer(offer.offerId, decision)); }}>Confirm decision</Button>
    </DialogActions></Dialog>
  </Stack>;
}
