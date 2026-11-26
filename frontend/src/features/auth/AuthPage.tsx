import { useState, type FormEvent } from 'react';
import { Alert, Box, Button, Link, MenuItem, Stack, Typography } from '@mui/material';
import { Link as RouterLink, Navigate, useLocation, useNavigate } from 'react-router';
import { register } from '../../api/auth';
import { TextInput } from '../../components/TextInput';
import { glass, tokens } from '../../theme';
import { useAuth } from './AuthProvider';
import { landingPath } from './routes';

export const passwordMismatch = 'Passwords do not match.';
const passwordRule = 'Password must be at least 8 characters and at most 72 bytes.';

// Login and register (redesign doc Section 5): headline left, glass form card right;
// on phones the headline comes first and the card takes the full width below it.
export function AuthPage({ registration = false }: { registration?: boolean }) {
  const { session, signIn } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState('');
  const [mismatch, setMismatch] = useState(false);
  const [busy, setBusy] = useState(false);
  if (session && session.expiresAt > Date.now()) return <Navigate replace to={landingPath(session.user.role)} />;
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const email = String(data.get('email'));
    const password = String(data.get('password'));
    setError(''); setMismatch(false);
    if (registration) {
      // Checked before sending; the confirm value itself never leaves the browser.
      if ([...password].length < 8 || new TextEncoder().encode(password).length > 72) { setError(passwordRule); return; }
      if (password !== String(data.get('confirmPassword'))) { setMismatch(true); setError(passwordMismatch); return; }
    }
    setBusy(true);
    try {
      if (registration) {
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
  // Both columns stay vertically centred in the space under the logo (about 116px with the main padding).
  // Centred as a pair with a fixed gap, so the headline and the form stay close on wide screens.
  return <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center',
    columnGap: 'clamp(80px, 10vw, 160px)', rowGap: 4, minHeight: { md: 'calc(100dvh - 116px)' } }}>
    {/* Capped so the headline keeps short lines on very wide screens. */}
    <Box sx={{ flex: '1 1 420px', minWidth: 0, maxWidth: 720 }}>
      {/* clamp(48px, 5vw, 76px), capped at 11vw below 440px so the longest word never scrolls sideways. */}
      <Typography component="p" sx={{ fontSize: 'min(clamp(48px, 5vw, 76px), 11vw)', fontWeight: 800, letterSpacing: '-0.045em', lineHeight: 1.04, color: tokens.text,
        textWrap: 'balance' }}>
        Jobs and internships, without the guesswork.
      </Typography>
      <Typography sx={{ mt: 2.5, fontSize: '1.25rem', lineHeight: 1.6, color: tokens.textSecondary, maxWidth: '44ch' }}>
        Search open roles, apply with a checked resume and follow every application from submitted to offer.
      </Typography>
    </Box>
    <Box sx={{ ...glass, flex: '0 1 440px', width: '100%', maxWidth: { xs: '100%', md: 440 }, borderRadius: '24px', p: { xs: 3, sm: 5 }, minWidth: 0 }}>
      <Stack component="form" onSubmit={submit} spacing={2.5} sx={{ minWidth: 0 }}>
        <Box>
          <Typography variant="h1" sx={{ fontSize: '2rem', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.2 }}>{registration ? 'Create account' : 'Log in'}</Typography>
          <Typography sx={{ mt: 0.75, fontSize: '0.9375rem', color: tokens.textSecondary }}>{registration ? 'Create your JobTrack account.' : 'Welcome back.'}</Typography>
        </Box>
        {location.state?.registered && !registration && <Alert severity="success">Account created. Log in to continue.</Alert>}
        {error && <Alert severity="error">{error}</Alert>}
        {registration && <TextInput name="fullName" label="Full name" autoComplete="name" required maxLength={100} />}
        <TextInput name="email" label="Email" type="email" autoComplete="email" required maxLength={254} />
        <TextInput name="password" label="Password" type="password" autoComplete={registration ? 'new-password' : 'current-password'} required
          helperText={registration ? 'At least 8 characters' : undefined} />
        {registration && <TextInput name="confirmPassword" label="Confirm password" type="password" autoComplete="new-password" required
          error={mismatch} helperText={mismatch ? passwordMismatch : undefined} onChange={() => { if (mismatch) setMismatch(false); }} />}
        {registration && <TextInput select name="role" label="Account type" defaultValue="ROLE_APPLICANT">
          <MenuItem value="ROLE_APPLICANT">Applicant</MenuItem><MenuItem value="ROLE_EMPLOYER">Employer</MenuItem>
        </TextInput>}
        <Button type="submit" variant="contained" fullWidth disabled={busy} sx={{ minHeight: 52, borderRadius: '14px', fontSize: '1rem' }}>
          {busy ? 'Please wait…' : registration ? 'Create account' : 'Log in'}
        </Button>
        <Typography sx={{ textAlign: 'center', fontSize: '0.9375rem', color: tokens.textSecondary }}>
          {registration ? 'Already have an account? ' : 'New here? '}
          <Link component={RouterLink} to={registration ? '/login' : '/register'} sx={{ color: tokens.accent }}>{registration ? 'Log in' : 'Create an account'}</Link>
        </Typography>
      </Stack>
    </Box>
  </Box>;
}
