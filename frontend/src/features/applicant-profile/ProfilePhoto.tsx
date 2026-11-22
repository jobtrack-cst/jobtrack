import { useEffect, useState } from 'react';
import { Avatar } from '@mui/material';
import { tokens } from '../../theme';
import { initialsOf } from '../employer/logo';
import { loadPhoto, onPhotoChange } from './photo';

interface Props {
  userId: string;
  name?: string;
  // false skips the request; undefined (unknown, as in the app bar) asks the gateway.
  hasPhoto?: boolean;
  size?: number;
}

// Shows the signed-in applicant's photo, or their initials when there is none
// or it fails to load (JT-103). Only the applicant's own photo is ever shown.
export function ProfilePhoto({ userId, name, hasPhoto, size = 40 }: Props) {
  const [src, setSrc] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  useEffect(() => onPhotoChange(() => setVersion(v => v + 1)), []);
  useEffect(() => {
    setSrc(null);
    if (hasPhoto === false) return;
    let current = true;
    void loadPhoto(userId).then(url => { if (current) setSrc(url); });
    return () => { current = false; };
  }, [userId, hasPhoto, version]);
  return <Avatar src={src ?? undefined} alt={name ? `${name} profile photo` : 'Profile photo'}
    // Initials use the Night accent circle, as the account button does (JT-110).
    sx={{ width: size, height: size, fontSize: size * 0.36, fontWeight: 800, bgcolor: tokens.accent, color: tokens.onAccent }}>{initialsOf(name)}</Avatar>;
}
