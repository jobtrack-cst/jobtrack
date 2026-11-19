import { useState } from 'react';
import { WorkspaceNavigation } from './components/WorkspaceNavigation';
import { editorialFont } from './theme';
import { Brand } from './components/Brand';
import { Landscape } from './components/Landscape';
import { WorkspaceIcon } from './components/WorkspaceIcon';
import { AppBar, Box, Button, Card, CardActionArea, CardContent, Container, LinearProgress, Stack, SvgIcon, Toolbar, Typography } from '@mui/material';
import { Link, Navigate, Outlet, Route, Routes, useLocation } from 'react-router';
import type { Role } from './api/auth';
import { AuthPage } from './features/auth/AuthPage';
import { useAuth } from './features/auth/AuthProvider';
import { landingPath } from './features/auth/routes';
import { ApplicationsPage } from './features/application/ApplicationsPage';
import { ApplyPage } from './features/application/ApplyPage';
import { OffersPage } from './features/offer/OffersPage';
import { MessagesPage } from './features/messaging/MessagesPage';
import { JobAlertsPage } from './features/job-alert/JobAlertsPage';
import { ProfilePage } from './features/applicant-profile/ProfilePage';
import { AccountMenu } from './features/auth/AccountMenu';
import { ResumesPage } from './features/resume/ResumesPage';
import { StatusPage } from './features/application-status/StatusPage';
import { ReviewPage } from './features/review/ReviewPage';
import { InterviewsPage } from './features/interview/InterviewsPage';
import { CompanyPage } from './features/employer/CompanyPage';
import { VerificationPage } from './features/company-verification/VerificationPage';
import { AdminVerificationPage } from './features/company-verification/AdminVerificationPage';
import { ReportJobPage } from './features/moderation/ReportJobPage';
import { AdminModerationPage } from './features/moderation/AdminModerationPage';
import { AnalyticsPage } from './features/analytics/AnalyticsPage';
import { AuditLogPage } from './features/audit-log/AuditLogPage';

import { JobsPage } from './features/job-posting/JobsPage';
import { ManageJobPage } from './features/job-posting/ManageJobPage';
import { SearchJobsPage } from './features/job-search/SearchJobsPage';
import { JobDetailsPage } from './features/job-search/JobDetailsPage';
import { SavedJobsPage } from './features/saved-jobs/SavedJobsPage';
import { NotificationsPage } from './features/notification-inbox/NotificationsPage';

// Workspace links per role; each path is relative to the role's landing path.
const navigation: Record<Role, [path: string, label: string][]> = {
  ROLE_APPLICANT: [['jobs', 'Search jobs'], ['saved-jobs', 'Saved jobs'], ['notifications', 'Notifications'], ['applications', 'Applications'], ['apply', 'Apply'], ['offers', 'Offers'], ['job-alerts', 'Job alerts'],
    ['profile', 'Profile'], ['resumes', 'Resumes'], ['report-job', 'Report a job']],
  ROLE_EMPLOYER: [['jobs', 'My jobs'], ['notifications', 'Notifications'], ['applications', 'Applications'], ['offers', 'Offers'], ['company', 'Company'], ['verification', 'Verification'],
    ['report-job', 'Report a job']],
  ROLE_ADMIN: [['notifications', 'Notifications'], ['verification', 'Verification'], ['moderation', 'Moderation'], ['analytics', 'Analytics'], ['audit-log', 'Audit log'],
    ['report-job', 'Report a job']],
};

function Workspace({ role, menuOpen, closeMenu }: { role: Role; menuOpen: boolean; closeMenu: () => void }) {
  const { session } = useAuth();
  const location = useLocation();
  if (!session || session.expiresAt <= Date.now()) return <Navigate to="/login" replace />;
  if (session.user.role !== role) return <Navigate to={landingPath(session.user.role)} replace />;
  const base = landingPath(role);
  const roleName = role.slice(5).toLowerCase();
  const active = navigation[role].find(([path]) => location.pathname === `${base}/${path}` || location.pathname.startsWith(`${base}/${path}/`));
  return <Box sx={{ display: { xs: 'block', md: 'grid' }, gridTemplateColumns: '232px minmax(0, 1fr)', gap: { md: 3, xl: 5 }, alignItems: 'start' }}>
    <WorkspaceNavigation base={base} roleName={roleName} items={navigation[role]} open={menuOpen} onClose={closeMenu} />
    <Box component="section" aria-label={`${roleName} content`} className="workspace-content" sx={{ minWidth: 0, overflowWrap: 'anywhere',
      '& h1': { mb: 0.5, animation: 'jt-reveal 180ms ease-out', maxWidth: '100%' },
      '& > .MuiStack-root > h1 + p': { color: 'text.secondary', maxWidth: '78ch' },
      '& form': { p: { xs: 2.5, sm: 3 }, bgcolor: 'background.paper', border: '1px solid', borderColor: 'divider', borderRadius: '14px',
        boxShadow: '0 1px 2px rgb(21 62 55 / 4%), 0 6px 18px rgb(21 62 55 / 3%)' },
      '& form form': { p: 0, border: 0, boxShadow: 'none' },
      '& .MuiStack-root > .MuiButton-root': { alignSelf: 'flex-start', maxWidth: '100%' },
      '& .MuiFormControl-root, & .MuiAutocomplete-root, & .MuiStack-root': { minWidth: 0 },
      '& .record-actions': { flexDirection: 'row', flexWrap: 'wrap', gap: 1, pt: 1.5, mt: 1, borderTop: '1px solid', borderColor: 'divider' },
      '& .record-id': { fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace', fontSize: '0.74rem', color: 'text.secondary' },
      '& .MuiTableContainer-root': { borderRadius: '12px' },
    }}>
      <Typography variant="overline" color="text.secondary" sx={{ display: 'block', mb: 1 }}>JobTrack <Box component="span" sx={{ mx: 1, color: 'secondary.main' }}>/</Box> {active?.[1] ?? 'Overview'}</Typography>
      <Outlet />
    </Box>
  </Box>;
}
const welcomeContent: Record<Role, { title: string; description: string; primary: [string, string]; secondary: [string, string]; links: [string, string, string][] }> = {
  ROLE_APPLICANT: {
    title: 'Find your next chapter.', description: 'Opportunity starts with a connection. Bring your experience, discover a role and keep every next step in view.',
    primary: ['jobs', 'Search jobs'], secondary: ['applications', 'View applications'],
    links: [['profile', 'Your profile & skills', 'Tell your story and keep your education and skills up to date.'], ['resumes', 'Your resumes', 'Prepare a validated resume for the opportunities ahead.'], ['saved-jobs', 'Saved opportunities', 'Return to the roles you want to explore.']],
  },
  ROLE_EMPLOYER: {
    title: 'Make room for great talent.', description: 'Build meaningful connections. Share your next opportunity and guide applicants through the hiring journey.',
    primary: ['jobs', 'My jobs'], secondary: ['applications', 'View applications'],
    links: [['company', 'Your company', 'Introduce your organisation to the JobTrack community.'], ['verification', 'Employer verification', 'View your verification requests and their decisions.'], ['offers', 'Your offers', 'Find an offer using its application or offer ID.']],
  },
  ROLE_ADMIN: {
    title: 'A trusted path to opportunity.', description: 'Keep the community moving forward. Review employers, follow hiring outcomes and support a fair recruitment experience.',
    primary: ['verification', 'Review employers'], secondary: ['analytics', 'View analytics'],
    links: [['moderation', 'Job reports', 'Review reported vacancies and take the appropriate action.'], ['audit-log', 'Audit trail', 'Inspect the events recorded across the platform.'], ['notifications', 'Notifications', 'Catch up on updates in your inbox.']],
  },
};
function WorkspaceWelcome({ role }: { role: Role }) {
  const { session } = useAuth();
  const label = role === 'ROLE_APPLICANT' ? 'Applicant' : role === 'ROLE_EMPLOYER' ? 'Employer' : 'Admin';
  const content = welcomeContent[role];
  const base = landingPath(role);
  return <Stack spacing={3}>
    <Typography variant="h4" component="h1">{label} workspace</Typography>
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', lg: '1.1fr .9fr' }, overflow: 'hidden',
      bgcolor: 'primary.dark', color: '#fff9ee', borderRadius: '18px', border: '1px solid #42614c', boxShadow: '0 8px 24px rgb(21 62 55 / 10%)' }}>
      <Stack spacing={2.5} sx={{ p: { xs: 3, sm: 4 }, minWidth: 0, alignItems: 'flex-start' }}>
        <Typography variant="overline" sx={{ color: '#e4c992' }}>Welcome, {session?.user.fullName}</Typography>
        <Typography component="h2" sx={{ fontFamily: editorialFont, fontSize: { xs: '2.1rem', sm: '2.7rem' }, lineHeight: 1.1, letterSpacing: '-.025em', maxWidth: '16ch' }}>{content.title}</Typography>
        <Typography sx={{ color: '#d9e3cf', maxWidth: 430 }}>{content.description}</Typography>
        <Stack direction="row" spacing={1.5} sx={{ flexWrap: 'wrap', pt: 1 }}>
          <Button component={Link} to={`${base}/${content.primary[0]}`} variant="contained" sx={{ bgcolor: '#eddfb8', color: '#254a39', '&:hover': { bgcolor: '#fff0c8' } }}>{content.primary[1]}</Button>
          <Button component={Link} to={`${base}/${content.secondary[0]}`} variant="outlined" sx={{ color: '#f6edcf', borderColor: '#94ae91', bgcolor: 'transparent', '&:hover': { bgcolor: '#325c48', borderColor: '#d8dfbd' } }}>{content.secondary[1]}</Button>
        </Stack>
      </Stack>
      <Box sx={{ p: { xs: 2, sm: 3 }, pl: { lg: 0 }, alignSelf: 'center', minWidth: 0 }}>
        <Box sx={{ height: { xs: 140, sm: 200, lg: 260 }, borderRadius: '12px', overflow: 'hidden', border: '1px solid #829977' }}><Landscape /></Box>
      </Box>
    </Box>
    <Typography variant="h6" component="h2">Your workspace, at a glance</Typography>
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', sm: 'repeat(3, minmax(0, 1fr))' }, gap: 2 }}>
      {content.links.map(([path, title, description]) => <Card key={path}>
        <CardActionArea component={Link} to={`${base}/${path}`} sx={{ height: '100%', '&.Mui-focusVisible': { outline: '3px solid #246657', outlineOffset: -3 } }}>
          <CardContent sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', height: '100%', gap: 1.5 }}>
            <Box sx={{ color: 'primary.main', display: 'grid', placeItems: 'center', width: 40, height: 40, bgcolor: '#e9eee0', border: '1px solid #d3deca', borderRadius: '10px' }}><WorkspaceIcon name={path} /></Box>
            <Typography variant="h6" component="h3">{title}</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>{description}</Typography>
            <Typography component="span" variant="button" color="primary.main">Open <Box component="span" aria-hidden="true" sx={{ ml: .5 }}>↗</Box></Typography>
          </CardContent>
        </CardActionArea>
      </Card>)}
    </Box>
  </Stack>;
}
function Home() {
  const { session } = useAuth();
  return <Navigate to={session && session.expiresAt > Date.now() ? landingPath(session.user.role) : '/login'} replace />;
}
export default function App() {
  const { session, initializing, signOut } = useAuth();
  const location = useLocation();
  // Associate the open drawer with this location, so navigation closes it without a delayed effect.
  const [menuLocation, setMenuLocation] = useState<typeof location | null>(null);
  const menuOpen = menuLocation === location;
  const closeMenu = () => setMenuLocation(null);
  const currentPage = session ? navigation[session.user.role].find(([path]) => location.pathname.startsWith(`${landingPath(session.user.role)}/${path}`))?.[1] ?? 'Overview' : '';

  if (initializing) return <Container component="main" sx={{ py: 8 }}><LinearProgress sx={{ mb: 3 }} /><Typography role="status">Restoring session…</Typography></Container>;
  return <>
    <Box component="a" href="#main-content" sx={{ position: 'fixed', top: -100, left: 16, zIndex: 1500, bgcolor: 'background.paper', p: 2,
      '&:focus': { top: 12 } }}>Skip to content</Box>
    <AppBar position="sticky" elevation={0} sx={{ bgcolor: '#fffefa', color: 'text.primary', borderBottom: '1px solid', borderColor: 'divider',
      '&::before': { content: '\'\'', height: 3, background: 'linear-gradient(90deg, #153e37 0%, #427459 46%, #bd9a5c 80%, #e9d8ad 100%)' } }}>
      <Toolbar sx={{ gap: { xs: 1, sm: 2 }, flexWrap: 'wrap', minHeight: { xs: 68, sm: 74 }, px: { xs: 1.5, sm: 4 } }}>
      <Brand to={session && session.expiresAt > Date.now() ? landingPath(session.user.role) : '/'} />
      <Box sx={{ flexGrow: 1 }} />
      {session && <AccountMenu key={session.user.userId} user={session.user} onSignOut={signOut} />}
    </Toolbar>
    {session && <Stack direction="row" alignItems="center" spacing={2} sx={{ display: { xs: 'flex', md: 'none' }, px: 2, pb: 1, borderTop: '1px solid #eef0e8' }}>
      <Button aria-label="Open navigation" aria-expanded={menuOpen} aria-controls={menuOpen ? 'workspace-navigation' : undefined} onClick={() => setMenuLocation(location)}
        startIcon={<SvgIcon><path d="M4 6h16M4 12h16M4 18h16" fill="none" stroke="currentColor" strokeWidth="1.7" /></SvgIcon>}>Menu</Button>
      <Typography variant="body2" color="text.secondary" noWrap sx={{ minWidth: 0 }}>{currentPage}</Typography>
    </Stack>}
    </AppBar>
    <Container component="main" id="main-content" tabIndex={-1} maxWidth="xl" sx={{ py: { xs: 3, sm: 4 }, px: { xs: 2, sm: 4 }, outline: 'none' }}>
      <Routes>
        <Route path="/login" element={<AuthPage key="login" />} />
        <Route path="/register" element={<AuthPage key="register" registration />} />
        <Route path="/applicant" element={<Workspace role="ROLE_APPLICANT" menuOpen={menuOpen} closeMenu={closeMenu} />}>
          <Route index element={<WorkspaceWelcome role="ROLE_APPLICANT" />} />
          <Route path="jobs" element={<SearchJobsPage />} />
          <Route path="jobs/:jobId" element={<JobDetailsPage />} />
          <Route path="saved-jobs" element={<SavedJobsPage />} />
          <Route path="notifications" element={<NotificationsPage key="applicant-notifications" />} />
          <Route path="apply" element={<ApplyPage />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="applications/:applicationId/status" element={<StatusPage />} />
          <Route path="applications/:applicationId/interviews" element={<InterviewsPage />} />
          <Route path="offers" element={<OffersPage key="applicant-offers" />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="job-alerts" element={<JobAlertsPage />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="resumes" element={<ResumesPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/applicant" replace />} />
        </Route>
        <Route path="/employer" element={<Workspace role="ROLE_EMPLOYER" menuOpen={menuOpen} closeMenu={closeMenu} />}>
          <Route index element={<WorkspaceWelcome role="ROLE_EMPLOYER" />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="applications/:applicationId/status" element={<StatusPage />} />
          <Route path="applications/:applicationId/review" element={<ReviewPage />} />
          <Route path="applications/:applicationId/interviews" element={<InterviewsPage />} />
          <Route path="offers" element={<OffersPage key="employer-offers" />} />
          <Route path="offers/new/:applicationId" element={<OffersPage key="make-offer" create />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="jobs" element={<JobsPage />} />
          <Route path="jobs/new" element={<ManageJobPage key="create-job" create />} />
          <Route path="jobs/:jobId" element={<ManageJobPage key="manage-job" />} />
          <Route path="notifications" element={<NotificationsPage key="employer-notifications" />} />
          <Route path="company" element={<CompanyPage />} />
          <Route path="verification" element={<VerificationPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/employer" replace />} />
        </Route>
        <Route path="/admin" element={<Workspace role="ROLE_ADMIN" menuOpen={menuOpen} closeMenu={closeMenu} />}>
          <Route index element={<WorkspaceWelcome role="ROLE_ADMIN" />} />
          <Route path="verification" element={<AdminVerificationPage />} />
          <Route path="notifications" element={<NotificationsPage key="admin-notifications" />} />
          <Route path="moderation" element={<AdminModerationPage />} />
          <Route path="analytics" element={<AnalyticsPage />} />
          <Route path="audit-log" element={<AuditLogPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Route>
        <Route path="*" element={<Home />} />
      </Routes>
    </Container>
  </>;
}
