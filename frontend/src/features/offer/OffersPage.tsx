import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, CardContent, Dialog, DialogActions, DialogTitle, Stack, TextField, Typography } from '@mui/material';
import { Link, useParams, useSearchParams } from 'react-router';
import { ApiError } from '../../api/client';
import { useAuth } from '../auth/AuthProvider';
import { errorMessage, validId } from '../application/shared';
import { getOffer, getOfferByApplication, makeOffer, respondOffer, type Offer } from './api';

export function OffersPage({ create = false }: { create?: boolean }) {
  const { applicationId = '' } = useParams();
  const [params] = useSearchParams();
  const queryApplicationId = params.get('applicationId') ?? '';
  const queryOfferId = params.get('offerId') ?? '';
  const applicant = useAuth().session?.user.role === 'ROLE_APPLICANT';
  const [id, setId] = useState(create ? applicationId : queryApplicationId || queryOfferId);
  const [byApplication, setByApplication] = useState(!!queryApplicationId);
  const request = useRef(0);
  const [missing, setMissing] = useState(false);
  const [terms, setTerms] = useState('');
  const [offer, setOffer] = useState<Offer>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [decision, setDecision] = useState<'accept' | 'decline'>();
  useEffect(() => {
    const current = ++request.current;
    setId(create ? applicationId : queryApplicationId || queryOfferId);
    setByApplication(!!queryApplicationId);
    setOffer(undefined); setError(''); setMissing(false); setDecision(undefined); setBusy(false);
    if (!create && queryApplicationId) {
      if (!validId(queryApplicationId)) { setError('Application ID must be a UUID.'); return; }
      setBusy(true);
      getOfferByApplication(queryApplicationId).then(result => {
        if (request.current === current) setOffer(result.offer);
      }).catch(reason => {
        if (request.current !== current) return;
        if (reason instanceof ApiError && reason.status === 404) setMissing(true);
        else setError(errorMessage(reason));
      }).finally(() => { if (request.current === current) setBusy(false); });
    }
    return () => { request.current++; };
  }, [create, applicationId, queryApplicationId, queryOfferId]);
  async function run(action: () => Promise<{ offer: Offer }>, byApplication = false, preserveOffer = false) {
    if (busy) return;
    const current = ++request.current;
    setBusy(true); setError(''); setMissing(false); if (!preserveOffer) setOffer(undefined);
    try {
      const result = await action();
      if (request.current === current) { setOffer(result.offer); setDecision(undefined); }
    } catch (reason) {
      if (request.current !== current) return;
      if (byApplication && reason instanceof ApiError && reason.status === 404) setMissing(true);
      else setError(errorMessage(reason));
    } finally { if (request.current === current) setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">{create ? 'Make offer' : 'View offer'}</Typography>
    {!create && <Typography>Open View Offer from an application, or look up an offer by application ID or offer ID.</Typography>}
    {create && <Typography>One offer per application is allowed. Terms must contain 1 to 2000 characters.</Typography>}
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); void run(() => create ? makeOffer(id.trim(), terms.trim()) : (byApplication ? getOfferByApplication(id.trim()) : getOffer(id.trim())), !create && byApplication); }}>
      <TextField label={create || byApplication ? 'Application ID' : 'Offer ID'} required value={id} disabled={busy} onChange={e => { setId(e.target.value); setOffer(undefined); setError(''); setMissing(false); }} />
      {create && <TextField label="Offer terms" required multiline minRows={4} value={terms} disabled={busy || !!offer} onChange={e => setTerms(e.target.value)} helperText={`${Array.from(terms.trim()).length}/2000 characters`} />}
      <Button variant="contained" type="submit" disabled={busy || !validId(id.trim()) || (create && (!!offer || !terms.trim() || Array.from(terms.trim()).length > 2000))}>{create ? 'Make offer' : 'Load offer'}</Button>
    </Stack>
    {!create && <Button disabled={busy} onClick={() => { setByApplication(!byApplication); setId(''); setOffer(undefined); setError(''); setMissing(false); }}>Use {byApplication ? 'offer' : 'application'} ID</Button>}
    {busy && <Typography role="status">Loading offer…</Typography>}
    {missing && <Alert severity="info">No offer has been made for this application.</Alert>}
    {error && <Alert severity="error">{error}</Alert>}
    {offer && <><Alert severity="success">{create ? 'Offer made. Offer ID: ' : 'Offer ID: '}{offer.offerId}</Alert>
      {create && <Button component={Link} to={`/employer/offers?applicationId=${encodeURIComponent(offer.applicationId)}`}>View Offer</Button>}
      <Card><CardContent><Stack spacing={2}>
        <Typography variant="h6" component="h2">Offer terms</Typography>
        <Typography className="record-id">Application: {offer.applicationId}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{offer.terms}</Typography><Typography variant="subtitle2" color="primary.dark">Status: {offer.status.replace('OFFER_STATUS_', '')}</Typography>
      </Stack></CardContent></Card>
      {applicant && offer.status === 'OFFER_STATUS_PENDING' && <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        <Button variant="contained" color="success" disabled={busy} onClick={() => setDecision('accept')}>Accept offer</Button><Button variant="outlined" color="error" disabled={busy} onClick={() => setDecision('decline')}>Decline offer</Button>
      </Stack>}</>}
    <Dialog open={!!decision} onClose={() => { if (!busy) setDecision(undefined); }}><DialogTitle>{decision === 'accept' ? 'Accept' : 'Decline'} this offer? This decision is final.</DialogTitle><DialogActions>
      <Button disabled={busy} onClick={() => setDecision(undefined)}>Cancel</Button><Button variant="contained" color={decision === 'decline' ? 'error' : 'success'} disabled={busy} onClick={() => { if (offer && decision) void run(() => respondOffer(offer.offerId, decision), false, true); }}>Confirm decision</Button>
    </DialogActions></Dialog>
  </Stack>;
}
