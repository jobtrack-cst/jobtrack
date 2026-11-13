import { useState, type FormEvent } from 'react';
import { Alert, Button, Link, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import { Link as RouterLink, Navigate, useLocation, useNavigate } from 'react-router';
import { register } from '../../api/auth';
import { useAuth } from './AuthProvider';
import { landingPath } from './routes';

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
  return <Paper sx={{ p: 3, maxWidth: 440, mx: 'auto' }}>
    <Stack component="form" onSubmit={submit} spacing={2}>
      <Typography variant="h4" component="h1">{registration ? 'Create account' : 'Log in'}</Typography>
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
  </Paper>;
}
