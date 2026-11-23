import { useState } from 'react';
import { Button } from '@mui/material';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { useAuth } from '../auth/AuthProvider';
import { saveJob, savedMessages, unsaveJob } from './api';
import { markSaved, useSavedIds } from './savedState';

// Save or Unsave one job. The page shows `error` and `notice` where it fits.
export function useSaveJob(jobId: string) {
  const userId = useAuth().session?.user.userId;
  const ids = useSavedIds(userId);
  const saved = ids?.has(jobId.toLowerCase()) ?? false;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function toggle() {
    if (busy || !userId) return;
    setBusy(true); setError(''); setNotice('');
    try {
      if (saved) await unsaveJob(jobId); else await saveJob(jobId);
      markSaved(userId, jobId, !saved);
      setNotice(saved ? 'Job removed from Saved.' : 'Job saved.');
    } catch (reason) {
      setError(gatewayMessage(reason, savedMessages));
      // The server already has the state the button offered; show that state.
      const code = (reason as { code?: number }).code;
      if (!saved && code === grpcCode.alreadyExists) markSaved(userId, jobId, true);
      if (saved && code === grpcCode.notFound) markSaved(userId, jobId, false);
    } finally { setBusy(false); }
  }
  return { saved, busy, error, notice, toggle };
}

export function SaveJobButton({ state, disabled, title }: { state: ReturnType<typeof useSaveJob>; disabled?: boolean; title?: string }) {
  const { saved, busy, toggle } = state;
  return <Button variant="outlined" color={saved ? 'error' : 'primary'} disabled={disabled || busy} onClick={() => void toggle()}
    aria-label={title ? `${saved ? 'Unsave' : 'Save'} ${title}` : undefined} sx={{ minHeight: 44, borderRadius: '12px', flexShrink: 0 }}>
    {saved ? 'Unsave' : 'Save'}
  </Button>;
}
