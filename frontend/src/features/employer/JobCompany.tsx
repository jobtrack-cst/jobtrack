import { useEffect, useState } from 'react';
import { Stack, Typography } from '@mui/material';
import { LogoAvatar } from './CompanyLogo';
import { loadCompany, type CompanyBadge } from './logo';

// Company logo and name on job cards and job details (JT-102). Initials replace
// a missing or failed logo; nothing is shown if the company profile cannot be read.
export function JobCompany({ employerId, size = 40 }: { employerId?: string; size?: number }) {
  const [company, setCompany] = useState<CompanyBadge | null>(null);
  useEffect(() => {
    setCompany(null);
    if (!employerId) return;
    let current = true;
    void loadCompany(employerId).then(c => { if (current) setCompany(c); });
    return () => { current = false; };
  }, [employerId]);
  if (!company) return null;
  return <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
    <LogoAvatar src={company.url} name={company.name} size={size} />
    <Typography variant="subtitle2" sx={{ minWidth: 0 }}>{company.name}</Typography>
  </Stack>;
}
