import { useEffect, useState } from 'react';
import { Avatar } from '@mui/material';
import { initialsOf, loadLogo } from './logo';

interface Props {
  employerId?: string;
  name?: string;
  // false skips the request; undefined (unknown) asks the gateway.
  hasLogo?: boolean;
  size?: number;
  // Changing it reloads the logo, for example after an upload.
  version?: number;
}

// The themed square used everywhere a company logo appears. MUI shows the
// initials when there is no src or the image fails to load.
export function LogoAvatar({ src, name, size = 40 }: { src?: string | null; name?: string; size?: number }) {
  return <Avatar variant="circular" src={src ?? undefined} alt={name ? `${name} logo` : 'Company logo'}
    sx={{ width: size, height: size, fontSize: size * 0.4, fontWeight: 700, flexShrink: 0,
      bgcolor: 'secondary.light', color: 'secondary.dark', border: '1px solid', borderColor: 'divider' }}>
    {initialsOf(name)}
  </Avatar>;
}

// Shows a company's logo, or its initials when there is none or it fails to load.
export function CompanyLogo({ employerId, name, hasLogo, size = 40, version = 0 }: Props) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    setSrc(null);
    if (!employerId || hasLogo === false) return;
    let current = true;
    void loadLogo(employerId).then(url => { if (current) setSrc(url); });
    return () => { current = false; };
  }, [employerId, hasLogo, version]);
  return <LogoAvatar src={src} name={name} size={size} />;
}
