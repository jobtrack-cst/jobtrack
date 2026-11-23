import { useEffect, useRef, useState } from 'react';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { LogoSquare } from '../../components/CompanyAvatar';
import { gatewayMessage, grpcCode } from '../application-status/shared';
import { putCompanyLogo, type CompanyProfile } from './api';
import { loadLogo, logoProblem, maxLogoBytes, rememberLogo, sizeProblem, toBase64 } from './logo';

const messages = {
  [grpcCode.invalidArgument]: 'The logo was not saved. Use a JPEG or PNG image of 1 to 1,000,000 bytes.',
  [grpcCode.permissionDenied]: 'Only the employer who owns this profile can change its logo.',
  [grpcCode.notFound]: 'Create your company profile before you add a logo.',
  413: 'The logo file is too large. Use an image of at most 1,000,000 bytes.',
};

interface Props { profile: CompanyProfile; onUploaded: (profile: CompanyProfile) => void }

// Logo upload on the Company page (JT-102 rules). The upload only changes the logo
// and updated_at, so an open profile edit is kept.
export function CompanyLogoUpload({ profile, onUploaded }: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [src, setSrc] = useState<string | null>(null);
  // Bumped after an upload, so the new logo is read from the shared cache.
  const [version, setVersion] = useState(0);
  const hasLogo = profile.hasLogo === true;
  useEffect(() => {
    setSrc(null);
    if (!hasLogo) return;
    let current = true;
    void loadLogo(profile.employerId).then(url => { if (current) setSrc(url); });
    return () => { current = false; };
  }, [profile.employerId, hasLogo, version]);

  async function upload(file?: File) {
    if (!file || busy) return;
    setError(''); setNotice('');
    // Checked before reading, so a very large file is never loaded into memory.
    if (file.size < 1 || file.size > maxLogoBytes) {
      setError(sizeProblem);
      if (input.current) input.current.value = '';
      return;
    }
    setBusy(true);
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      const problem = logoProblem(file.type, bytes);
      if (problem) { setError(problem); return; }
      const content = toBase64(bytes);
      const result = await putCompanyLogo(file.type, content);
      rememberLogo(result.profile.employerId, file.type, content);
      setVersion(v => v + 1);
      onUploaded(result.profile);
      setNotice(hasLogo ? 'Logo replaced.' : 'Logo added.');
    } catch (reason) {
      setError(gatewayMessage(reason, messages));
    } finally {
      setBusy(false);
      // Lets the same file be chosen again after an error.
      if (input.current) input.current.value = '';
    }
  }

  return <Card component="section" label="Company logo">
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1.5 }}>
        <LogoSquare src={src} name={profile.companyName} size={72} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="h3" component="h2">Company logo</Typography>
          <Typography variant="body2" color="text.secondary">JPEG or PNG, up to 1,000,000 bytes. Without a logo, the company initials are shown.</Typography>
        </Box>
        <Button variant="outlined" component="label" disabled={busy}>
          {busy ? 'Uploading…' : hasLogo ? 'Replace logo' : 'Upload logo'}
          <input ref={input} hidden type="file" accept="image/jpeg,image/png" onChange={e => void upload(e.target.files?.[0])} />
        </Button>
      </Stack>
      {error && <Alert severity="error">{error}</Alert>}
      {notice && <Alert severity="success" role="status">{notice}</Alert>}
    </Stack>
  </Card>;
}
