// Browser calls go directly to the JT-53 gateway; never send cookies.
const gateway = 'http://localhost:8000';
let accessToken: string | null = null;
let unauthorized = () => {};
let authGeneration = 0;

export function configureAuth(token: string | null, onUnauthorized: () => void) {
  authGeneration++;
  accessToken = token;
  unauthorized = onUnauthorized;
}

export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code?: number) {
    // Do not display arbitrary server bodies, which may contain sensitive input.
    super(status === 401 ? 'Invalid credentials or session expired. Please log in.'
      : status === 409 ? 'An account with this email already exists.'
      : status === 400 ? 'Check your details and try again.'
      : status === 403 ? 'You do not have permission for this action.'
      : 'The request failed. Please try again.');
  }
}

export async function api<T>(path: string, options: { method?: string; body?: unknown; public?: boolean } = {}): Promise<T> {
  if (!path.startsWith('/v1/') || path.includes('..')) throw new Error('Invalid API path');
  const token = accessToken;
  const generation = authGeneration;
  const headers: Record<string, string> = {};
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';
  if (!options.public && token) headers.Authorization = `Bearer ${token}`;
  let response: Response;
  try {
    response = await fetch(`${gateway}${path}`, {
      method: options.method ?? 'GET', headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: 'omit', redirect: 'error',
    });
  } catch {
    throw new Error('Cannot reach JobTrack. Please try again.');
  }
  if (!response.ok) {
    // A late response from an old session must not log out a newer login.
    if (response.status === 401 && !options.public && generation === authGeneration) unauthorized();
    let code: number | undefined;
    try {
      const body: unknown = await response.json();
      if (body && typeof body === 'object' && 'code' in body && typeof body.code === 'number') code = body.code;
    } catch { /* HTTP status remains usable without a JSON body. */ }
    throw new ApiError(response.status, code);
  }
  if (response.status === 204) return undefined as T;
  try { return await response.json() as T; }
  catch { throw new Error('JobTrack returned an invalid response. Please try again.'); }
}
