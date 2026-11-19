// JT-48 upload rules, checked in the browser before anything is sent.
export const maxResumeBytes = 2_000_000;
export const maxFileNameLength = 255;
const pdfSignature = [0x25, 0x50, 0x44, 0x46, 0x2d]; // "%PDF-"
// The resume service rejects these in a file name: slash, backslash and NUL (JT-48).
const forbiddenInFileName = ['/', '\\', '\u0000'];

export type Checked = { ok: true; fileName: string; content: string } | { ok: false; message: string };

export async function checkResume(file: File): Promise<Checked> {
  const fileName = file.name.trim();
  const nameLength = Array.from(fileName).length;
  if (nameLength < 1 || nameLength > maxFileNameLength || forbiddenInFileName.some(c => fileName.includes(c))) {
    return { ok: false, message: `The file name must have 1 to ${maxFileNameLength} characters and no slashes.` };
  }
  if (file.type !== 'application/pdf') return { ok: false, message: 'Only PDF files can be uploaded.' };
  if (file.size < 1) return { ok: false, message: 'The file is empty.' };
  if (file.size > maxResumeBytes) return { ok: false, message: 'The file is larger than 2 MB (2,000,000 bytes).' };
  const bytes = new Uint8Array(await file.arrayBuffer());
  if (!pdfSignature.every((b, i) => bytes[i] === b)) return { ok: false, message: 'This file is not a valid PDF.' };
  return { ok: true, fileName, content: toBase64(bytes) };
}

function toBase64(bytes: Uint8Array) {
  let binary = '';
  // Chunks keep String.fromCharCode below the argument limit for 2 MB files.
  for (let i = 0; i < bytes.length; i += 0x8000) binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(binary);
}
