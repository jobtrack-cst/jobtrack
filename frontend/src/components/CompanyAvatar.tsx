import { useEffect, useState } from 'react';
import { Avatar, Stack, Typography } from '@mui/material';
import { initialsOf, loadCompany, type CompanyBadge } from '../features/employer/logo';
import { tokens } from '../theme';

// Company name and logo from the shared per-session cache in employer/logo.ts
// (GetCompanyProfile, then GetCompanyLogo only when the profile has a logo).
export function useCompany(employerId?: string) {
  const [state, setState] = useState<{ id?: string; company: CompanyBadge | null; loading: boolean }>({ company: null, loading: !!employerId });
  useEffect(() => {
    if (!employerId) { setState({ company: null, loading: false }); return; }
    let current = true;
    setState({ id: employerId, company: null, loading: true });
    void loadCompany(employerId).then(company => { if (current) setState({ id: employerId, company, loading: false }); });
    return () => { current = false; };
  }, [employerId]);
  return state.id === employerId ? state : { company: null, loading: !!employerId };
}

// Avatars are circles everywhere (JT-110).
export function LogoSquare({ src, name, size = 44 }: { src?: string | null; name?: string; size?: number }) {
  // Decorative: the company name is always shown as text beside it.
  return <Avatar variant="circular" src={src ?? undefined} alt="" aria-hidden="true"
    sx={{ width: size, height: size, borderRadius: '50%', fontSize: size * 0.38, fontWeight: 800, flexShrink: 0,
      bgcolor: tokens.chipBg, color: tokens.chipText, border: `1px solid ${tokens.border}` }}>
    {initialsOf(name)}
  </Avatar>;
}

// Logo, or initials on chipBg when there is none. With showName the company name sits beside it.
export function CompanyAvatar({ employerId, size = 44, showName = false }: { employerId?: string; size?: number; showName?: boolean }) {
  const { company } = useCompany(employerId);
  if (!showName) return <LogoSquare src={company?.url} name={company?.name} size={size} />;
  return <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
    <LogoSquare src={company?.url} name={company?.name} size={size} />
    <Typography color="text.secondary" sx={{ minWidth: 0 }}>{company?.name || 'Company'}</Typography>
  </Stack>;
}
