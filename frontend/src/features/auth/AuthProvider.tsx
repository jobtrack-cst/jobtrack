import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { currentUser, login, roles, type User } from '../../api/auth';
import { configureAuth } from '../../api/client';

interface Session { user: User; expiresAt: number }
interface Auth { session: Session | null; signIn: (email: string, password: string) => Promise<void>; signOut: () => void }
const Context = createContext<Auth | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  function signOut() {
    configureAuth(null, signOut);
    setSession(null);
  }
  async function signIn(email: string, password: string) {
    const result = await login(email, password);
    const expiresAt = Date.parse(result.expiresAt);
    if (!result.accessToken || !Number.isFinite(expiresAt) || expiresAt <= Date.now()
      || expiresAt > Date.now() + 3600_000 + 60_000) throw new Error('Invalid login response. Please log in again.');
    configureAuth(result.accessToken, signOut);
    try {
      // The protected endpoint confirms the session and role with the gateway.
      const { user } = await currentUser();
      if (!user?.userId || !roles.includes(user.role)) throw new Error('Unsupported account role.');
      setSession({ user, expiresAt });
    } catch (error) {
      signOut();
      throw error;
    }
  }
  useEffect(() => {
    if (!session) return;
    const timer = window.setTimeout(signOut, Math.max(0, session.expiresAt - Date.now()));
    const checkExpiry = () => { if (Date.now() >= session.expiresAt) signOut(); };
    window.addEventListener('focus', checkExpiry);
    document.addEventListener('visibilitychange', checkExpiry);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('focus', checkExpiry);
      document.removeEventListener('visibilitychange', checkExpiry);
    };
  }, [session]);
  return <Context.Provider value={{ session, signIn, signOut }}>{children}</Context.Provider>;
}

export function useAuth() {
  const auth = useContext(Context);
  if (!auth) throw new Error('AuthProvider is required');
  return auth;
}
