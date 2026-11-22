import { useState, type KeyboardEvent, type MouseEvent } from 'react';
import { Box, Button, Divider, ListItemIcon, ListSubheader, Menu, MenuItem, Typography } from '@mui/material';
import { Link, useLocation } from 'react-router';
import type { User } from '../../api/auth';
import { Icon } from '../../components/Icon';
import { tokens } from '../../theme';
import { ProfilePhoto } from '../applicant-profile/ProfilePhoto';
import { initialsOf } from '../employer/logo';

export const roleLabel = (role: User['role']) => ({ ROLE_APPLICANT: 'Applicant', ROLE_EMPLOYER: 'Employer', ROLE_ADMIN: 'Admin' })[role];

// Name, role and Log out (redesign doc Section 3).
export function AccountMenu({ user, onSignOut }: { user: User; onSignOut: () => void }) {
  const location = useLocation();
  // Navigation closes the menu immediately, including browser back and forward.
  const [menu, setMenu] = useState<{ anchor: HTMLElement; location: typeof location } | null>(null);
  const open = menu?.location === location;
  const close = () => setMenu(null);
  const name = user.fullName?.trim() || 'Your account';
  const role = roleLabel(user.role);
  const destination = user.role === 'ROLE_APPLICANT' ? { path: '/applicant/profile', label: 'My profile' }
    : user.role === 'ROLE_EMPLOYER' ? { path: '/employer/company', label: 'Company profile' } : undefined;
  function show(event: MouseEvent<HTMLButtonElement> | KeyboardEvent<HTMLButtonElement>) {
    setMenu({ anchor: event.currentTarget, location });
  }
  return <>
    {/* A 44px circle: the applicant's own photo when there is one (JT-103), otherwise
        accent initials. Name and role are in the accessible name and the menu. */}
    <Button id="account-menu-button" aria-label={`Account menu for ${name}, ${role}`} aria-haspopup="menu"
      aria-expanded={open} aria-controls={open ? 'account-menu' : undefined} onClick={show}
      onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); show(event); } }}
      sx={{ minWidth: 44, width: 44, height: 44, minHeight: 44, p: 0, flexShrink: 0, borderRadius: '50%', overflow: 'hidden',
        bgcolor: tokens.accent, color: tokens.onAccent, fontWeight: 800, fontSize: '0.9375rem',
        boxShadow: open ? `0 0 0 3px ${tokens.accentHover}` : 'none', '&:hover': { bgcolor: tokens.accentHover },
        '&.Mui-focusVisible': { outline: `3px solid ${tokens.accent}`, outlineOffset: 2 } }}>
      {user.role === 'ROLE_APPLICANT'
        ? <ProfilePhoto userId={user.userId} name={user.fullName} size={44} />
        : <Box component="span" aria-hidden="true">{initialsOf(user.fullName)}</Box>}
    </Button>
    <Menu id="account-menu" anchorEl={open ? menu?.anchor : null} open={open} onClose={close} transitionDuration={160}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      slotProps={{ list: { 'aria-labelledby': 'account-menu-button' }, paper: { sx: { width: 280, maxWidth: 'calc(100vw - 24px)' } } }}>
      <ListSubheader disableSticky role="presentation" sx={{ px: 2, py: 1.25, bgcolor: 'transparent', lineHeight: 'normal', whiteSpace: 'normal' }}>
        <Typography variant="subtitle1" color="text.primary" sx={{ overflowWrap: 'anywhere' }}>{name}</Typography>
        <Typography variant="body2" color="text.secondary">{role} account</Typography>
      </ListSubheader>
      <Divider sx={{ my: 0.5 }} />
      {destination && <MenuItem component={Link} to={destination.path} onClick={close}>
        <ListItemIcon><Icon name={user.role === 'ROLE_APPLICANT' ? 'profile' : 'company'} size={18} /></ListItemIcon>{destination.label}
      </MenuItem>}
      <MenuItem component="button" type="button" onClick={() => { close(); onSignOut(); }} sx={{ width: 'calc(100% - 12px)', color: 'error.main' }}>
        <ListItemIcon sx={{ color: 'inherit' }}><Icon name="logout" size={18} /></ListItemIcon>Log out
      </MenuItem>
    </Menu>
  </>;
}
