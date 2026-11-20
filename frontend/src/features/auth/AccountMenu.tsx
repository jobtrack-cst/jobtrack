import { useState, type MouseEvent, type KeyboardEvent } from 'react';
import { Box, Button, Divider, ListItemIcon, ListSubheader, Menu, MenuItem, Stack, SvgIcon, Typography } from '@mui/material';
import { Link, useLocation } from 'react-router';
import type { User } from '../../api/auth';
import { WorkspaceIcon } from '../../components/WorkspaceIcon';
import { ProfilePhoto } from '../applicant-profile/ProfilePhoto';
import { initialsOf } from '../employer/logo';

export function AccountMenu({ user, onSignOut }: { user: User; onSignOut: () => void }) {
  const location = useLocation();
  // Navigation closes the menu immediately, including browser back/forward.
  const [menu, setMenu] = useState<{ anchor: HTMLElement; location: typeof location } | null>(null);
  const open = menu?.location === location;
  const close = () => setMenu(null);
  const name = user.fullName?.trim() || 'Your account';
  const role = { ROLE_APPLICANT: 'Applicant', ROLE_EMPLOYER: 'Employer', ROLE_ADMIN: 'Admin' }[user.role];
  const destination = user.role === 'ROLE_APPLICANT' ? { path: '/applicant/profile', label: 'My profile', icon: 'profile' }
    : user.role === 'ROLE_EMPLOYER' ? { path: '/employer/company', label: 'Company profile', icon: 'company' } : undefined;
  function show(event: MouseEvent<HTMLButtonElement> | KeyboardEvent<HTMLButtonElement>) {
    setMenu({ anchor: event.currentTarget, location });
  }
  return <>
    <Button id="account-menu-button" aria-label={`Account menu for ${name}, ${role}`} aria-haspopup="menu"
      aria-expanded={open} aria-controls={open ? 'account-menu' : undefined} onClick={show}
      onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); show(event); } }}
      color="inherit" endIcon={<SvgIcon aria-hidden="true" sx={{ fontSize: 16, transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 160ms ease' }}>
        <path d="m6 9 6 6 6-6" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
      </SvgIcon>}
      sx={{ minWidth: 0, minHeight: 44, maxWidth: { xs: 146, sm: 270 }, px: { xs: .75, sm: 1.25 }, py: .625, border: '1px solid',
        borderColor: open ? '#91aa91' : '#d4dfd0', bgcolor: open ? '#e9f0e3' : '#fffefa', borderRadius: '10px',
        boxShadow: open ? '0 0 0 3px rgb(36 102 87 / 7%)' : '0 1px 3px rgb(21 62 55 / 3%)',
        '&:hover': { bgcolor: '#eff4e9', borderColor: '#8ca58c' },
        '& .MuiButton-endIcon': { ml: .5, mr: 0 },
        '&.Mui-focusVisible': { outline: '3px solid #246657', outlineOffset: 3 },
      }}>
      <Stack component="span" direction="row" spacing={1} alignItems="center" sx={{ minWidth: 0 }}>
        <Box component="span" aria-hidden="true" sx={{ flexShrink: 0 }}>
          {user.role === 'ROLE_APPLICANT' ? <ProfilePhoto userId={user.userId} name={user.fullName} size={32} />
            : <Box component="span" sx={{ width: 32, height: 32, display: 'grid', placeItems: 'center', borderRadius: '50%', bgcolor: '#e4eee1',
              color: 'primary.dark', border: '1px solid #cfddc9', fontSize: '.72rem', fontWeight: 700 }}>{initialsOf(user.fullName)}</Box>}
        </Box>
        <Stack component="span" spacing={0} sx={{ textAlign: 'left', minWidth: 0 }}>
          <Typography component="span" variant="subtitle2" noWrap sx={{ display: { xs: 'none', sm: 'block' }, maxWidth: 176 }}>{name}</Typography>
          <Typography component="span" variant="caption" sx={{ color: 'text.secondary', fontSize: '.7rem' }}>{role}</Typography>
        </Stack>
      </Stack>
    </Button>
    <Menu id="account-menu" anchorEl={open ? menu?.anchor : null} open={open} onClose={close} transitionDuration={140}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      slotProps={{ list: { 'aria-labelledby': 'account-menu-button', sx: { '& .MuiMenuItem-root': { minHeight: 44 } } }, paper: { sx: { mt: 1, width: 280, maxWidth: 'calc(100vw - 24px)',
        border: '1px solid #ccd8c6', borderTop: '3px solid #bd9a5c', borderRadius: '12px' } } }}>
      <ListSubheader disableSticky role="presentation" sx={{ p: 1.5, bgcolor: 'transparent', lineHeight: 'normal', whiteSpace: 'normal' }}>
        <Typography variant="subtitle2" color="text.primary" sx={{ overflowWrap: 'anywhere' }}>{name}</Typography>
        <Typography variant="caption" color="text.secondary">{role} account</Typography>
      </ListSubheader>
      <Divider sx={{ my: 1 }} />
      {destination && <MenuItem component={Link} to={destination.path} onClick={close}>
        <ListItemIcon><WorkspaceIcon name={destination.icon} /></ListItemIcon>{destination.label}
      </MenuItem>}
      <MenuItem component="button" type="button" onClick={() => { close(); onSignOut(); }} sx={{ width: 'calc(100% - 8px)', color: 'error.main' }}>
        <ListItemIcon sx={{ color: 'inherit' }}><SvgIcon aria-hidden="true" sx={{ fontSize: 18 }}>
          <path d="M9 4H4v16h5m5-13 5 5-5 5m-6-5h11" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
        </SvgIcon></ListItemIcon>Log out
      </MenuItem>
    </Menu>
  </>;
}
