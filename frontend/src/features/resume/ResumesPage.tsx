import { useRef, useState } from 'react';
import { Alert, Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { Pager, ResourceState, useResource } from '../application/shared';
import { formatTime, gatewayMessage, grpcCode } from '../application-status/shared';
import { listMyResumes, uploadResume, type ValidationStatus } from './api';
import { checkResume } from './upload';

const statusView: Record<ValidationStatus, { label: string; color: 'default' | 'warning' | 'success' | 'error' }> = {
  RESUME_VALIDATION_STATUS_UNSPECIFIED: { label: 'Unknown', color: 'default' },
  RESUME_VALIDATION_STATUS_PENDING: { label: 'Pending validation', color: 'warning' },
  RESUME_VALIDATION_STATUS_VALIDATED: { label: 'Validated', color: 'success' },
  RESUME_VALIDATION_STATUS_REJECTED: { label: 'Rejected', color: 'error' },
};
const uploadMessages = {
  413: 'The file is too large. Resumes can be at most 2 MB (2,000,000 bytes).',
  [grpcCode.invalidArgument]: 'The resume was not accepted. Upload a PDF of at most 2 MB (2,000,000 bytes) with a file name of up to 255 characters.',
  [grpcCode.permissionDenied]: 'Only applicants can upload resumes.',
};
const sizeText = (size?: string | number) => size === undefined ? '' : `${(Number(size) / 1_000_000).toFixed(2)} MB`;

export function ResumesPage() {
  const [token, setToken] = useState('');
  const resource = useResource(() => listMyResumes(token), [token]);
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function upload(file: File | undefined) {
    if (!file || busy) return;
    setBusy(true); setError(''); setNotice('');
    try {
      const checked = await checkResume(file);
      if (!checked.ok) { setError(checked.message); return; }
      await uploadResume(checked.fileName, checked.content);
      setNotice(`${checked.fileName} uploaded. It shows as pending until it is validated; refresh to see the result.`);
      setToken(''); resource.refresh();
    } catch (reason) { setError(gatewayMessage(reason, uploadMessages)); }
    finally { setBusy(false); if (input.current) input.current.value = ''; }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">My resumes</Typography>
    <Typography>Upload a PDF of at most 2 MB. Each resume is checked after upload; only validated resumes can be used to apply.</Typography>
    <Button variant="contained" component="label" disabled={busy}>
      {busy ? 'Uploading...' : 'Upload PDF resume'}
      <input ref={input} hidden type="file" accept="application/pdf" name="resume" onChange={e => void upload(e.target.files?.[0])} />
    </Button>
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    <Button disabled={busy || resource.loading} onClick={resource.refresh}>Refresh resumes</Button>
    <ResourceState {...resource} />
    {resource.data && !resource.data.resumes?.length && <Typography>No resumes on this page.</Typography>}
    {resource.data?.resumes?.map(r => { const view = statusView[r.validationStatus] ?? statusView.RESUME_VALIDATION_STATUS_UNSPECIFIED; return <Card key={r.resumeId}><CardContent><Stack spacing={1}>
      <Typography variant="h6" component="h2">{r.fileName}</Typography>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}><Chip label={view.label} color={view.color} /><Typography>{sizeText(r.sizeBytes)}</Typography></Stack>
      {r.validationStatus === 'RESUME_VALIDATION_STATUS_REJECTED' && <Typography>Reason: {r.rejectionReason || 'Not given'}</Typography>}
      <Typography>Uploaded: {formatTime(r.timestamps?.createdAt)}</Typography>
      <Typography variant="body2">Resume ID: {r.resumeId}</Typography>
    </Stack></CardContent></Card>; })}
    <Pager next={resource.data?.pagination?.nextPageToken} token={token} busy={busy || resource.loading} onPage={setToken} />
  </Stack>;
}
