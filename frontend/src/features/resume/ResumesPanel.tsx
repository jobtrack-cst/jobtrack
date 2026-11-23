import { useRef, useState } from 'react';
import { Alert, Box, Button, Link, Stack, Typography } from '@mui/material';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { ListPanel, ListRow, ResultsHeader } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { Pager } from '../../components/Pager';
import { StatusChip } from '../../components/StatusChip';
import { shortDate } from '../../components/format';
import { useResource } from '../application/shared';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { downloadResume, listMyResumes, uploadResume, type Resume } from './api';
import { checkResume } from './upload';
import { usePdfOpener } from './usePdfOpener';

const uploadMessages = {
  413: 'The file is too large. Resumes can be at most 2 MB (2,000,000 bytes).',
  [grpcCode.invalidArgument]: 'The resume was not accepted. Upload a PDF of at most 2 MB (2,000,000 bytes) with a file name of up to 255 characters.',
  [grpcCode.permissionDenied]: 'Only applicants can upload resumes.',
};
const viewMessages = {
  [grpcCode.notFound]: 'This resume is no longer available. Refresh the list.',
  [grpcCode.permissionDenied]: 'You can only open your own resumes.',
};
const sizeText = (size?: string | number) => size === undefined ? '' : `${(Number(size) / 1_000_000).toFixed(2)} MB`;

function ResumeRow({ resume }: { resume: Resume }) {
  const { busy, error, fallback, open } = usePdfOpener(viewMessages);
  const view = () => open(() => downloadResume(resume.resumeId), resume.fileName);
  const rejected = resume.validationStatus === 'RESUME_VALIDATION_STATUS_REJECTED';
  const uploaded = shortDate(resume.timestamps?.createdAt);
  return <ListRow>
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ alignItems: { sm: 'center' }, justifyContent: 'space-between' }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography sx={{ fontWeight: 700 }}>{resume.fileName}</Typography>
        <Typography variant="body2" color="text.secondary">{[sizeText(resume.sizeBytes), uploaded && `Uploaded ${uploaded}`].filter(Boolean).join('. ')}</Typography>
        {rejected && <Typography variant="body2" sx={{ mt: 0.5 }}>Reason: {resume.rejectionReason || 'Not given'}</Typography>}
      </Box>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexShrink: 0 }}>
        <StatusChip kind="resume" status={resume.validationStatus} />
        <Button variant="outlined" disabled={busy} onClick={() => void view()} aria-label={`View PDF ${resume.fileName}`}>{busy ? 'Opening…' : 'View PDF'}</Button>
      </Stack>
    </Stack>
    {fallback && <Alert severity="info" sx={{ mt: 1.5 }}>Your browser blocked the new tab. <Link href={fallback.url} download={resume.fileName}>Download PDF</Link></Alert>}
    {error && <Alert severity="error" sx={{ mt: 1.5 }}>{error}</Alert>}
  </ListRow>;
}

// Resumes tab of the profile: upload a PDF, see each one's check status, open it.
export function ResumesPanel() {
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
      setNotice(`${checked.fileName} uploaded. It shows as checking until it is validated; refresh to see the result.`);
      setToken(''); resource.refresh();
    } catch (reason) { setError(gatewayMessage(reason, uploadMessages)); }
    finally { setBusy(false); if (input.current) input.current.value = ''; }
  }
  const resumes = resource.data?.resumes ?? [];
  const total = Number(resource.data?.pagination?.totalCount ?? resumes.length);
  return <Stack spacing={2.5}>
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ alignItems: { sm: 'center' }, justifyContent: 'space-between' }}>
      <Typography color="text.secondary" sx={{ maxWidth: '60ch' }}>Upload a PDF of at most 2 MB. Each resume is checked after upload; only validated resumes can be used to apply.</Typography>
      <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
        <Button disabled={busy || resource.loading} onClick={resource.refresh}>Refresh</Button>
        <Button variant="contained" component="label" disabled={busy}>
          {busy ? 'Uploading…' : 'Upload PDF'}
          <input ref={input} hidden type="file" accept="application/pdf" name="resume" onChange={e => void upload(e.target.files?.[0])} />
        </Button>
      </Stack>
    </Stack>
    {error && <Alert severity="error">{error}</Alert>}
    {notice && <Alert severity="success">{notice}</Alert>}
    {resource.loading && <LoadingRows rows={2} label="Loading resumes" />}
    {resource.error && <ErrorState message={resource.error} onRetry={resource.refresh} />}
    {resource.data && !resumes.length && <EmptyState title="No resumes yet" description="Upload a PDF resume to apply for jobs." />}
    {!!resumes.length && <>
      <ResultsHeader title={`${total.toLocaleString()} ${total === 1 ? 'resume' : 'resumes'}`} />
      <ListPanel label="Resumes">{resumes.map(r => <ResumeRow key={r.resumeId} resume={r} />)}</ListPanel>
    </>}
    {resource.data && <Pager token={token} nextToken={resource.data.pagination?.nextPageToken} totalCount={resource.data.pagination?.totalCount}
      count={resumes.length} busy={busy || resource.loading} onPage={setToken} />}
  </Stack>;
}
