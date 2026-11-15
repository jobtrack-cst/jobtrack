import { ApiError } from '../../api/client';

// gRPC codes that grpc-gateway returns in the JSON error body.
export const grpcCode = { invalidArgument: 3, notFound: 5, alreadyExists: 6, permissionDenied: 7, failedPrecondition: 9 } as const;

// Maps a failed gateway call to a fixed message. The server's own message is
// never shown, because it can repeat what the user typed.
export function gatewayMessage(error: unknown, byCode: Partial<Record<number, string>> = {}) {
  if (error instanceof ApiError) {
    if (error.status === 413) return byCode[413] ?? 'The request is too large.';
    const code = error.code ?? (error.status === 404 ? grpcCode.notFound : undefined);
    if (code !== undefined && byCode[code]) return byCode[code];
    if (code === grpcCode.notFound) return 'This record was not found or is not available yet. Refresh shortly.';
  }
  return error instanceof Error ? error.message : 'The request failed. Please try again.';
}

export const isNotFound = (error: unknown) => error instanceof ApiError && (error.code === grpcCode.notFound || error.status === 404);

// Shows a timestamp from the gateway in the browser's time zone.
export function formatTime(value?: string) {
  if (!value) return 'Not set';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? 'Not set' : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

// Counts characters as the services do (code points, after trimming).
export const length = (value: string) => Array.from(value.trim()).length;
