import { AppBar, Button, Container, Stack, Toolbar, Typography } from '@mui/material';
import { Link, Navigate, Outlet, Route, Routes } from 'react-router';
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

function Workspace({ role }: { role: Role }) {
  const { session } = useAuth();
  if (!session || session.expiresAt <= Date.now()) return <Navigate to="/login" replace />;
  if (session.user.role !== role) return <Navigate to={landingPath(session.user.role)} replace />;
  const base = landingPath(role);
  return <Stack spacing={3}>
    {role !== 'ROLE_ADMIN' && <Stack component="nav" aria-label="Workspace" direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}>
      <Button component={Link} to={`${base}/applications`}>Applications</Button>
      {role === 'ROLE_APPLICANT' && <Button component={Link} to={`${base}/apply`}>Apply</Button>}
      <Button component={Link} to={`${base}/offers`}>Offers</Button>
      {role === 'ROLE_APPLICANT' && <Button component={Link} to={`${base}/job-alerts`}>Job alerts</Button>}
      {role === 'ROLE_APPLICANT' && <Button component={Link} to={`${base}/profile`}>Profile</Button>}
    </Stack>}
    <Outlet />
  </Stack>;
}
function Home() {
  const { session } = useAuth();
  return <Navigate to={session && session.expiresAt > Date.now() ? landingPath(session.user.role) : '/login'} replace />;
}
export default function App() {
  const { session, signOut } = useAuth();
  return <>
    <AppBar position="static"><Toolbar>
      <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>JobTrack</Typography>
      {session && <Button color="inherit" onClick={signOut}>Log out</Button>}
    </Toolbar></AppBar>
    <Container component="main" sx={{ py: 4 }}>
      <Routes>
        <Route path="/login" element={<AuthPage key="login" />} />
        <Route path="/register" element={<AuthPage key="register" registration />} />
        <Route path="/applicant" element={<Workspace role="ROLE_APPLICANT" />}>
          <Route index element={<Typography variant="h4" component="h1">Applicant workspace</Typography>} />
          <Route path="apply" element={<ApplyPage />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="offers" element={<OffersPage key="applicant-offers" />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="job-alerts" element={<JobAlertsPage />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="*" element={<Navigate to="/applicant" replace />} />
        </Route>
        <Route path="/employer" element={<Workspace role="ROLE_EMPLOYER" />}>
          <Route index element={<Typography variant="h4" component="h1">Employer workspace</Typography>} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="offers" element={<OffersPage key="employer-offers" />} />
          <Route path="offers/new/:applicationId" element={<OffersPage key="make-offer" create />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="*" element={<Navigate to="/employer" replace />} />
        </Route>
        <Route path="/admin/*" element={<Workspace role="ROLE_ADMIN" />}>
          <Route path="*" element={<Typography variant="h4" component="h1">Admin workspace</Typography>} />
        </Route>
        <Route path="*" element={<Home />} />
      </Routes>
    </Container>
  </>;
}
