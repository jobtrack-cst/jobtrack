import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { currentUser, login, roles, type User } from '../../api/auth';
import { configureAuth } from '../../api/client';

interface Session { user: User; expiresAt: number }
interface StoredSession { accessToken: string; expiresAt: number }
interface Auth { session: Session | null; initializing: boolean; signIn: (email: string, password: string) => Promise<void>; signOut: () => void }
const Context = createContext<Auth | null>(null);
const storageKey = 'jobtrack.auth';

function validCredentials(value: unknown): value is StoredSession {
  if (!value || typeof value !== 'object') return false;
  const { accessToken, expiresAt } = value as Partial<StoredSession>;
  return typeof accessToken === 'string' && /^[\x21-\x7e]+$/.test(accessToken)
    && typeof expiresAt === 'number' && Number.isFinite(expiresAt)
    && expiresAt > Date.now() && expiresAt <= Date.now() + 3600_000 + 60_000;
}
function clearStorage() {
  try { window.sessionStorage.removeItem(storageKey); }
  catch { /* Storage may be disabled; in-memory logout still applies. */ }
}
function readStorage(): StoredSession | null {
  try {
    const raw = window.sessionStorage.getItem(storageKey);
    if (raw === null) return null;
    const value: unknown = JSON.parse(raw);
    if (validCredentials(value)) return { accessToken: value.accessToken, expiresAt: value.expiresAt };
  } catch { /* Treat inaccessible or malformed storage as unauthenticated. */ }
  clearStorage();
  return null;
}
function saveStorage(credentials: StoredSession) {
  try { window.sessionStorage.setItem(storageKey, JSON.stringify(credentials)); }
  catch { clearStorage(); /* Login still works in memory when storage is unavailable. */ }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [initializing, setInitializing] = useState(true);
  const generation = useRef(0);
  const signOut = useCallback(() => {
    generation.current++;
    configureAuth(null, signOut);
    clearStorage();
    setSession(null);
    setInitializing(false);
  }, []);
  const confirmSession = useCallback(async (credentials: StoredSession, attempt: number) => {
    configureAuth(credentials.accessToken, signOut);
    // Identity and role always come from the protected gateway endpoint.
    const { user } = await currentUser();
    if (generation.current !== attempt) throw new Error('Session changed. Please log in again.');
    if (typeof user?.userId !== 'string' || !user.userId.trim() || !roles.includes(user.role)) throw new Error('Unsupported account role.');
    if (!validCredentials(credentials)) throw new Error('Session expired. Please log in again.');
    saveStorage(credentials);
    setSession({ user, expiresAt: credentials.expiresAt });
  }, [signOut]);
  async function signIn(email: string, password: string) {
    signOut();
    const attempt = generation.current;
    try {
      const result = await login(email, password);
      if (generation.current !== attempt) throw new Error('Session changed. Please log in again.');
      const credentials = { accessToken: result.accessToken, expiresAt: Date.parse(result.expiresAt) };
      if (!validCredentials(credentials)) throw new Error('Invalid login response. Please log in again.');
      await confirmSession(credentials, attempt);
    } catch (error) {
      if (generation.current === attempt) signOut();
      throw error;
    }
  }
  useEffect(() => {
    const attempt = ++generation.current;
    const credentials = readStorage();
    if (!credentials) signOut();
    else {
      void confirmSession(credentials, attempt).catch(() => {
        if (generation.current === attempt) signOut();
      }).finally(() => {
        if (generation.current === attempt) setInitializing(false);
      });
    }
    return () => {
      generation.current++;
      configureAuth(null, () => {});
    };
  }, [confirmSession, signOut]);
  useEffect(() => {
    if (!session) return;
    const attempt = generation.current;
    const checkExpiry = () => {
      if (generation.current === attempt && Date.now() >= session.expiresAt) signOut();
    };
    const timer = window.setTimeout(checkExpiry, Math.max(0, session.expiresAt - Date.now()));
    window.addEventListener('focus', checkExpiry);
    document.addEventListener('visibilitychange', checkExpiry);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('focus', checkExpiry);
      document.removeEventListener('visibilitychange', checkExpiry);
    };
  }, [session, signOut]);
  return <Context.Provider value={{ session, initializing, signIn, signOut }}>{children}</Context.Provider>;
}

export function useAuth() {
  const auth = useContext(Context);
  if (!auth) throw new Error('AuthProvider is required');
  return auth;
}
