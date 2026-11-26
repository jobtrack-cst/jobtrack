import { useEffect, useState, type ReactNode } from 'react';
import { AppBar, Box, Button, Drawer, IconButton, Stack, Toolbar, useMediaQuery, useTheme } from '@mui/material';
import { NavLink, useLocation, useNavigationType } from 'react-router';
import type { Role, User } from '../api/auth';
import { AccountMenu, roleLabel } from '../features/auth/AccountMenu';
import { landingPath } from '../features/auth/routes';
import { glass, tokens } from '../theme';
import { Brand } from './Brand';
import { Icon, type IconName } from './Icon';

// Main navigation per role (redesign doc Section 3). Paths are relative to the
// role's landing path; '' is Home. Pages not rebuilt yet keep their current routes.
// Messages joins both menus with the inbox in Phase 4 (JT-113).
export const navigation: Record<Role, [path: string, label: string, icon: IconName][]> = {
  ROLE_APPLICANT: [['', 'Home', 'home'], ['jobs', 'Find jobs', 'search'], ['saved', 'Saved', 'bookmark'], ['applications', 'Applications', 'applications'],
    ['messages', 'Messages', 'messages'], ['profile', 'Profile', 'profile'], ['job-alerts', 'Job alerts', 'alerts']],
  ROLE_EMPLOYER: [['', 'Home', 'home'], ['jobs', 'Jobs', 'jobs'], ['applicants', 'Applicants', 'people'], ['messages', 'Messages', 'messages'],['company', 'Company', 'company']],
  ROLE_ADMIN: [['', 'Home', 'home'], ['verification', 'Verification', 'verification'], ['moderation', 'Moderation', 'moderation'],
    ['analytics', 'Analytics', 'analytics'], ['audit-log', 'Audit log', 'audit']],
};

export const contentWidth = 1180;
// Signed-out pages: full width with a small gutter, so the logo sits near the top-left
// corner and lines up with the content instead of floating inside a capped column.
const publicContainer = { width: '100%', px: 'clamp(16px, 3vw, 48px)', boxSizing: 'border-box' } as const;

// Nav links are pills: 15px, 600, radius 12; the active one is filled white.
const pillSx = {
  flexShrink: 0, justifyContent: 'flex-start', borderRadius: '12px', fontSize: '0.9375rem', fontWeight: 600, color: tokens.textSecondary,
  py: 1.25, minHeight: 44, transition: 'background-color 180ms ease, color 180ms ease',
  '&:hover': { color: tokens.text, bgcolor: 'rgba(255,255,255,0.08)' },
  '&[aria-current="page"], &[aria-current="page"]:hover': { bgcolor: '#FFFFFF', color: tokens.onAccent },
} as const;

function NavItems({ role, vertical, onNavigate }: { role: Role; vertical: boolean; onNavigate?: () => void }) {
  const base = landingPath(role);
  return <Stack component="nav" id={vertical ? 'workspace-navigation' : undefined} aria-label="Main" direction={vertical ? 'column' : 'row'} spacing={vertical ? 0.75 : 0.5}
    sx={vertical ? undefined : { minWidth: 0, overflowX: 'auto', scrollbarWidth: 'none', py: 0.25 }}>
    {navigation[role].map(([path, label, icon]) => <Button key={path || 'home'} component={NavLink} to={path ? `${base}/${path}` : base} end={!path} onClick={onNavigate}
      startIcon={vertical ? <Icon name={icon} /> : undefined}
      // 16px side padding from 1200px up; 12px between 900 and 1200 so seven links still fit.
      sx={{ ...pillSx, px: vertical ? 2 : { md: 1.5, lg: 2 }, fontSize: vertical ? '1rem' : pillSx.fontSize }}>{label}</Button>)}
  </Stack>;
}

// Floating glass top bar with logo, role navigation and account button. Under 900px
// the navigation moves into a drawer (MUI Drawer: focus trap, Escape, focus return).
// Signed-out pages show only the logo, top left, on the background.
export function AppShell({ user, onSignOut, children }: { user?: User; onSignOut: () => void; children: ReactNode }) {
  const location = useLocation();
  // Tie the open drawer to this location, so navigation closes it without a delayed effect.
  const [menuLocation, setMenuLocation] = useState<typeof location | null>(null);
  const open = menuLocation === location;
  const close = () => setMenuLocation(null);
  const home = user ? landingPath(user.role) : '/login';
  // Render one navigation only, so there is never a hidden duplicate "Main" landmark.
  const theme = useTheme();
  const desktop = useMediaQuery(theme.breakpoints.up('md'), { noSsr: true });
  // A drawer left open on a phone must not reappear after the window grows and shrinks again.
  useEffect(() => { if (desktop) setMenuLocation(null); }, [desktop]);
  // A new page starts at the top, so its header and back link are not left under the sticky
  // top bar at the previous page's scroll position (JT-115 item 2). Back and Forward (POP)
  // keep the browser's own restoration; query changes such as ?tab= keep the position.
  const navigationType = useNavigationType();
  useEffect(() => { if (navigationType !== 'POP') window.scrollTo(0, 0); }, [location.pathname, navigationType]);
  return <>
    <Box component="a" href="#main-content" sx={{ position: 'fixed', top: -100, left: 16, zIndex: 1500, px: 2, py: 1.5, borderRadius: '12px',
      bgcolor: tokens.accent, color: tokens.onAccent, fontWeight: 700, textDecoration: 'none', '&:focus': { top: 12 } }}>Skip to content</Box>
    {user
      ? <AppBar position="sticky" elevation={0} color="transparent" sx={{ ...glass, top: 20, mt: '20px', mx: 'auto', width: { xs: 'calc(100% - 32px)', sm: 'calc(100% - 64px)' },
        maxWidth: contentWidth, borderRadius: '18px', color: 'text.primary' }}>
        {/* Wraps onto a second row when text is enlarged (200%) on a narrow phone. */}
        <Toolbar disableGutters sx={{ gap: { xs: 1, md: 1.5 }, flexWrap: 'wrap', minHeight: 'auto !important', p: '10px 14px 10px 18px' }}>
          <Brand to={home} />
          {desktop ? <Box sx={{ display: 'flex', flex: 1, minWidth: 0, justifyContent: 'center' }}><NavItems role={user.role} vertical={false} /></Box>
            : <Box sx={{ flex: 1 }} />}
          <AccountMenu key={user.userId} user={user} onSignOut={onSignOut} />
          {/* Always mounted and hidden with CSS from 900px, so it never remounts while being clicked. */}
          <Button aria-label="Open navigation" aria-expanded={open} aria-controls={open ? 'workspace-navigation' : undefined}
            onClick={() => setMenuLocation(location)} startIcon={<Icon name="menu" />} color="inherit"
            sx={{ display: { xs: 'inline-flex', md: 'none' }, flexShrink: 0, px: 1.5, borderRadius: '12px', border: `1px solid ${tokens.glassBorder}`,
              // Icon only on the narrowest phones, so the bar fits at 320px.
              '@media (max-width: 399.95px)': { minWidth: 44, px: 1.25, '& .MuiButton-startIcon': { m: 0 }, '& .menu-label': { display: 'none' } } }}>
            <span className="menu-label">Menu</span></Button>
        </Toolbar>
      </AppBar>
      : <Box component="header" sx={{ ...publicContainer, pt: 3 }}>
        <Brand to={home} large />
      </Box>}
    {user && <Drawer anchor="right" open={open && !desktop} onClose={close} transitionDuration={180}
      slotProps={{ paper: { role: 'dialog', 'aria-modal': true, 'aria-label': `${roleLabel(user.role)} navigation`, sx: { width: 'min(320px, calc(100vw - 32px))', p: 2 } } }}>
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Brand to={home} onClick={close} />
        <IconButton aria-label="Close navigation" onClick={close} sx={{ color: 'text.primary' }}><Icon name="close" /></IconButton>
      </Stack>
      <NavItems role={user.role} vertical onNavigate={close} />
    </Drawer>}
    <Box component="main" id="main-content" tabIndex={-1} sx={user
      ? { maxWidth: contentWidth, mx: 'auto', px: { xs: 2, sm: 4 }, pt: { xs: 4, sm: 6 }, pb: 8, boxSizing: 'content-box', outline: 'none' }
      : { ...publicContainer, py: { xs: 4, md: 3 }, outline: 'none' }}>
      {children}
    </Box>
  </>;
}
