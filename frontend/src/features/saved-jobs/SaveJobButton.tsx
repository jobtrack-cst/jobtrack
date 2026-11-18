import { useState } from 'react';
import { Alert, Button, Stack } from '@mui/material';
import { gatewayMessage } from '../application-status/shared';
import { saveJob, savedMessages, unsaveJob } from './api';

export function SaveJobButton({ jobId }: { jobId: string }) {
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function change() {
    if (busy) return;
    setBusy(true); setError('');
    try { if (saved) await unsaveJob(jobId); else await saveJob(jobId); setSaved(!saved); }
    catch (reason) { setError(gatewayMessage(reason, savedMessages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={1}><Button disabled={busy} onClick={() => void change()}>{saved ? 'Unsave job' : 'Save job'}</Button>
    {saved && <Alert severity="success">Job saved.</Alert>}{error && <Alert severity="error">{error}</Alert>}
  </Stack>;
}
