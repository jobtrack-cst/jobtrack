import { length } from '../application-status/shared';

// JT-52 rules: strictly after now, at most 365 days ahead, details 0 to 1000
// characters where only newline, carriage return and tab are allowed controls.
export const maxDetails = 1000;
export const maxDaysAhead = 365;
const dayMs = 24 * 60 * 60 * 1000;

// value comes from <input type="datetime-local"> and is the user's local time.
export function checkSchedule(value: string, details: string, now = new Date()): { ok: true; scheduledAt: string } | { ok: false; message: string } {
  const when = value ? new Date(value) : undefined;
  if (!when || Number.isNaN(when.getTime())) return { ok: false, message: 'Choose a date and time.' };
  if (when.getTime() <= now.getTime()) return { ok: false, message: 'Choose a time in the future.' };
  if (when.getTime() > now.getTime() + maxDaysAhead * dayMs) return { ok: false, message: `Choose a time within ${maxDaysAhead} days.` };
  if (length(details) > maxDetails) return { ok: false, message: `Details can have at most ${maxDetails} characters.` };
  if (Array.from(details).some(c => { const n = c.codePointAt(0)!; return (n < 32 && c !== '\n' && c !== '\r' && c !== '\t') || n === 127; })) {
    return { ok: false, message: 'Details contain a character that is not allowed.' };
  }
  return { ok: true, scheduledAt: when.toISOString() };
}
