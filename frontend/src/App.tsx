import { AppBar, Button, Container, Stack, Toolbar, Typography } from '@mui/material';
import { Navigate, Route, Routes } from 'react-router';
import type { Role } from './api/auth';
import { AuthPage } from './features/auth/AuthPage';
import { useAuth } from './features/auth/AuthProvider';
import { landingPath } from './features/auth/routes';

function Landing({ role, title }: { role: Role; title: string }) {
  const { session } = useAuth();
  if (!session || session.expiresAt <= Date.now()) return <Navigate to="/login" replace />;
  if (session.user.role !== role) return <Navigate to={landingPath(session.user.role)} replace />;
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">{title}</Typography>
    <Typography>Welcome, {session.user.fullName}.</Typography>
    <Typography>Your JobTrack workspace is ready. Feature pages will be added here.</Typography>
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
        <Route path="/applicant/*" element={<Landing role="ROLE_APPLICANT" title="Applicant workspace" />} />
        <Route path="/employer/*" element={<Landing role="ROLE_EMPLOYER" title="Employer workspace" />} />
        <Route path="/admin/*" element={<Landing role="ROLE_ADMIN" title="Admin workspace" />} />
        <Route path="*" element={<Home />} />
      </Routes>
    </Container>
  </>;
}
