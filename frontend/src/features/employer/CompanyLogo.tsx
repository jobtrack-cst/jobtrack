import { useEffect, useState } from 'react';
import { Avatar } from '@mui/material';
import { initialsOf, loadLogo } from './logo';

interface Props {
  employerId?: string;
  name?: string;
  // false skips the request; undefined (unknown, as on job cards) asks the gateway.
  hasLogo?: boolean;
  size?: number;
  // Changing it reloads the logo, for example after an upload.
  version?: number;
}

// Shows a company's logo, or its initials when there is none or it fails to
// load. Job cards (JT-73) can reuse it with the job's employer_id.
export function CompanyLogo({ employerId, name, hasLogo, size = 40, version = 0 }: Props) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    setSrc(null);
    if (!employerId || hasLogo === false) return;
    let current = true;
    void loadLogo(employerId).then(url => { if (current) setSrc(url); });
    return () => { current = false; };
  }, [employerId, hasLogo, version]);
  return <Avatar variant="rounded" src={src ?? undefined} alt={name ? `${name} logo` : 'Company logo'}
    sx={{ width: size, height: size, fontSize: size * 0.4 }}>{initialsOf(name)}</Avatar>;
}
