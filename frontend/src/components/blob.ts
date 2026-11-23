// Bytes from the gateway arrive as base64 (how grpc-gateway carries proto bytes in JSON).
// The caller chooses the type, so only types it has checked ever become a Blob.
export function base64ToBlob(base64: string, type: string) {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new Blob([bytes], { type });
}
