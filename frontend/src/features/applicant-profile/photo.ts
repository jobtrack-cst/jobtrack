import { isNotFound } from '../application-status/shared';
import { getMyProfilePhoto } from './api';

// Same rules as the applicant-profile service (JT-103); the service stays authoritative.
export const maxPhotoBytes = 1_000_000;
const signatures: Record<string, readonly number[]> = {
  'image/jpeg': [0xff, 0xd8, 0xff],
  'image/png': [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a],
};
const isPhotoType = (type: string) => type === 'image/jpeg' || type === 'image/png';
export const photoSizeProblem = 'The photo must be 1 to 1,000,000 bytes.';

// Returns an empty string when the file may be sent, otherwise a fixed message.
export function photoProblem(type: string, bytes: Uint8Array) {
  if (bytes.length < 1 || bytes.length > maxPhotoBytes) return photoSizeProblem;
  if (!isPhotoType(type)) return 'The photo must be a JPEG or PNG file.';
  if (!signatures[type].every((b, i) => bytes[i] === b)) return 'The file is not a real JPEG or PNG image. Save it again as JPEG or PNG and retry.';
  return '';
}

// Only validated image types become data URLs, so nothing else reaches an img src.
function dataUrl(type: string, base64?: string) {
  return isPhotoType(type) && base64 ? `data:${type};base64,${base64}` : null;
}

// One request per signed-in applicant, shared by the app bar and the profile
// page. "No photo" is remembered; other failures are retried on the next load.
let cached: { userId: string; request: Promise<string | null> } | null = null;
const listeners = new Set<() => void>();

export function loadPhoto(userId: string): Promise<string | null> {
  if (cached?.userId === userId) return cached.request;
  const request: Promise<string | null> = getMyProfilePhoto().then(
    r => dataUrl(r.contentType ?? '', r.content),
    (error: unknown) => {
      if (!isNotFound(error) && cached?.request === request) cached = null;
      return null;
    });
  cached = { userId, request };
  return request;
}

// After an upload every shown photo updates without fetching it again.
export function rememberPhoto(userId: string, type: string, base64: string) {
  cached = { userId, request: Promise.resolve(dataUrl(type, base64)) };
  listeners.forEach(listener => listener());
}

export function onPhotoChange(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
