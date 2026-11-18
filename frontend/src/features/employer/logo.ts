import { isNotFound } from '../application-status/shared';
import { getCompanyLogo } from './api';

// Same rules as the employer service (JT-102); the service stays authoritative.
export const maxLogoBytes = 1_000_000;
export const logoTypes = ['image/jpeg', 'image/png'] as const;
export type LogoType = typeof logoTypes[number];
const signatures: Record<LogoType, readonly number[]> = {
  'image/jpeg': [0xff, 0xd8, 0xff],
  'image/png': [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a],
};

export const isLogoType = (type: string): type is LogoType => (logoTypes as readonly string[]).includes(type);
export const sizeProblem = 'The logo must be 1 to 1,000,000 bytes.';

// Returns an empty string when the file may be sent, otherwise a fixed message.
export function logoProblem(type: string, bytes: Uint8Array) {
  if (bytes.length < 1 || bytes.length > maxLogoBytes) return sizeProblem;
  if (!isLogoType(type)) return 'The logo must be a JPEG or PNG file.';
  if (!signatures[type].every((b, i) => bytes[i] === b)) return 'The file is not a real JPEG or PNG image. Save it again as JPEG or PNG and retry.';
  return '';
}

// Converts in chunks, because one call with a million arguments can overflow the stack.
export function toBase64(bytes: Uint8Array) {
  let binary = '';
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...Array.from(bytes.subarray(i, i + 0x8000)));
  }
  return btoa(binary);
}

// Up to two initials from the company name, shown when there is no logo.
export function initialsOf(name?: string) {
  const words = (name ?? '').trim().split(/\s+/).filter(Boolean);
  return words.slice(0, 2).map(w => (Array.from(w)[0] ?? '').toUpperCase()).join('') || '?';
}

// Only validated image types become data URLs, so nothing else reaches an img src.
function dataUrl(type: string, base64?: string) {
  return isLogoType(type) && base64 ? `data:${type};base64,${base64}` : null;
}

// One request per employer per session, so a list of job cards from the same
// employer loads the logo once. "No logo" is remembered; other failures are retried.
const cache = new Map<string, Promise<string | null>>();

export function loadLogo(employerId: string): Promise<string | null> {
  const cached = cache.get(employerId);
  if (cached) return cached;
  const request = getCompanyLogo(employerId).then(
    r => dataUrl(r.contentType ?? '', r.content),
    (error: unknown) => {
      if (!isNotFound(error)) cache.delete(employerId);
      return null;
    });
  cache.set(employerId, request);
  return request;
}

// After an upload the new logo is shown without fetching it again.
export function rememberLogo(employerId: string, type: string, base64: string) {
  cache.set(employerId, Promise.resolve(dataUrl(type, base64)));
}
