import { useState, type FormEvent } from 'react';
import { Alert, Box, Button, Link, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import { Link as RouterLink, Navigate, useLocation, useNavigate } from 'react-router';
import { register } from '../../api/auth';
import { useAuth } from './AuthProvider';
import { landingPath } from './routes';
import { Landscape } from '../../components/Landscape';
import { editorialFont } from '../../theme';

export function AuthPage({ registration = false }: { registration?: boolean }) {
  const { session, signIn } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  if (session && session.expiresAt > Date.now()) return <Navigate replace to={landingPath(session.user.role)} />;
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const email = String(data.get('email'));
    const password = String(data.get('password'));
    setError('');
    setBusy(true);
    try {
      if (registration) {
        if ([...password].length < 8 || new TextEncoder().encode(password).length > 72) {
          throw new Error('Password must be at least 8 characters and at most 72 bytes.');
        }
        await register(email, password, String(data.get('fullName')), data.get('role') === 'ROLE_EMPLOYER' ? 'ROLE_EMPLOYER' : 'ROLE_APPLICANT');
        form.reset();
        navigate('/login', { state: { registered: true } });
      } else {
        await signIn(email, password);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Please try again.');
    } finally { setBusy(false); }
  }
  return <Box className="auth-layout" sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: '1fr 1fr' }, maxWidth: 1050, mx: 'auto', mt: { xs: 0, md: 2 },
    border: '1px solid #cdd7c9', borderRadius: '20px', overflow: 'hidden', bgcolor: 'background.paper', boxShadow: '0 3px 6px rgb(21 62 55 / 4%), 0 20px 64px rgb(21 62 55 / 8%)' }}>
    <Box className="auth-story" sx={{ order: { xs: 1, md: 0 }, bgcolor: 'primary.dark', color: '#fff9ee', minWidth: 0, display: 'flex', flexDirection: 'column',
      backgroundImage: 'radial-gradient(ellipse at top right, #355d47, transparent 80%)' }}>
      <Stack spacing={{ xs: 2, md: 3 }} sx={{ p: { xs: 3, sm: 4, md: 5 }, pb: { xs: 2, md: 3 } }}>
        <Typography variant="overline" sx={{ color: '#e3c896' }}>CST · Careers & opportunities</Typography>
        <Typography component="p" sx={{ fontFamily: editorialFont, fontSize: { xs: '2rem', md: '3rem' }, lineHeight: 1.1, letterSpacing: '-0.03em', maxWidth: '15ch' }}>
          Your next chapter,<br /><Box component="span" sx={{ color: '#e4c68e', fontStyle: 'italic' }}>closer to home.</Box>
        </Typography>
        <Typography sx={{ color: '#e0e7d5', maxWidth: 350, fontSize: { xs: '.875rem', md: '.9375rem' } }}>Discover opportunities, connect with employers and take your next step with JobTrack.</Typography>
      </Stack>
      <Box component="figure" className="auth-illustration" sx={{ m: 0, mx: { xs: 3, md: 4 }, height: { xs: 96, sm: 130, md: 210 },
        borderRadius: '12px', overflow: 'hidden', border: '1px solid #879d7c' }}><Landscape /></Box>
      <Box className="auth-story-footer" sx={{ mt: 'auto', pt: 2.5 }}>
        <Typography variant="caption" sx={{ display: 'block', color: '#d9e4cf', px: { xs: 3, md: 5 }, pb: 2 }}>College of Science and Technology · Bhutan</Typography>
        <Box aria-hidden="true" sx={{ height: 10, backgroundImage: 'url(/branding/weave.svg)', backgroundRepeat: 'repeat-x', backgroundPosition: 'center' }} />
      </Box>
    </Box>
    <Paper sx={{ minWidth: 0, p: { xs: 3, sm: 5, md: 6 }, borderRadius: 0, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
    <Stack component="form" sx={{ minWidth: 0 }} onSubmit={submit} spacing={2.5}>
      <Typography variant="h4" component="h1">{registration ? 'Create account' : 'Log in'}</Typography>
      <Typography color="text.secondary">{registration ? 'Start your journey with JobTrack.' : 'Welcome back. Continue to your workspace.'}</Typography>
      {location.state?.registered && !registration && <Alert severity="success">Account created. Log in to continue.</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      {registration && <TextField name="fullName" label="Full name" autoComplete="name" required slotProps={{ htmlInput: { maxLength: 100 } }} />}
      <TextField name="email" label="Email" type="email" autoComplete="email" required slotProps={{ htmlInput: { maxLength: 254 } }} />
      <TextField name="password" label="Password" type="password" autoComplete={registration ? 'new-password' : 'current-password'} required />
      {registration && <TextField select name="role" label="Account type" defaultValue="ROLE_APPLICANT">
        <MenuItem value="ROLE_APPLICANT">Applicant</MenuItem><MenuItem value="ROLE_EMPLOYER">Employer</MenuItem>
      </TextField>}
      <Button type="submit" variant="contained" disabled={busy}>{busy ? 'Please wait…' : registration ? 'Register' : 'Log in'}</Button>
      <Link component={RouterLink} to={registration ? '/login' : '/register'}>{registration ? 'Already have an account? Log in' : 'Create an account'}</Link>
    </Stack>
    </Paper>
  </Box>;
}
