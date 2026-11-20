import { useRef, useState } from 'react';
import { Alert, Button, Stack, Typography } from '@mui/material';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { toBase64 } from '../employer/logo';
import { putMyProfilePhoto, type ApplicantProfile } from './api';
import { ProfilePhoto } from './ProfilePhoto';
import { maxPhotoBytes, photoProblem, photoSizeProblem, rememberPhoto } from './photo';

const messages = {
  [grpcCode.invalidArgument]: 'The photo was not saved. Use a JPEG or PNG image of 1 to 1,000,000 bytes.',
  [grpcCode.permissionDenied]: 'Only applicants can have a profile photo.',
  [grpcCode.notFound]: 'Create your profile before you add a photo.',
  413: 'The photo file is too large. Use an image of at most 1,000,000 bytes.',
};

interface Props { userId: string; name?: string; hasPhoto: boolean; onUploaded: (profile: ApplicantProfile) => void }

export function ProfilePhotoUpload({ userId, name, hasPhoto, onUploaded }: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  async function upload(file?: File) {
    if (!file || busy) return;
    setError(''); setNotice('');
    // Checked before reading, so a very large file is never loaded into memory.
    if (file.size < 1 || file.size > maxPhotoBytes) {
      setError(photoSizeProblem);
      if (input.current) input.current.value = '';
      return;
    }
    setBusy(true);
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      const problem = photoProblem(file.type, bytes);
      if (problem) { setError(problem); return; }
      const content = toBase64(bytes);
      const result = await putMyProfilePhoto(file.type, content);
      rememberPhoto(userId, file.type, content);
      onUploaded(result.profile);
      setNotice(hasPhoto ? 'Photo replaced.' : 'Photo added.');
    } catch (reason) {
      setError(gatewayMessage(reason, messages));
    } finally {
      setBusy(false);
      // Lets the same file be chosen again after an error.
      if (input.current) input.current.value = '';
    }
  }

  return <Stack spacing={1.5} sx={{ p: 2.5, bgcolor: 'background.paper', border: '1px solid', borderColor: 'divider', borderRadius: '14px' }}>
    <Typography variant="h6" component="h2">Profile photo</Typography>
    <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
      <ProfilePhoto userId={userId} name={name} hasPhoto={hasPhoto} size={72} />
      <Button variant="outlined" component="label" disabled={busy}>
        {busy ? 'Uploading' : hasPhoto ? 'Replace photo' : 'Upload photo'}
        <input ref={input} hidden type="file" name="photo" accept="image/jpeg,image/png"
          onChange={e => void upload(e.target.files?.[0])} />
      </Button>
    </Stack>
    <Typography variant="body2">JPEG or PNG, up to 1,000,000 bytes. Without a photo, your initials are shown.</Typography>
    {error && <Alert severity="error">{error}</Alert>}
    {notice && <Alert severity="success">{notice}</Alert>}
  </Stack>;
}
