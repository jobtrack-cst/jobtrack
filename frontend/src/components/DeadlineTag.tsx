import { Chip } from '@mui/material';
import { tokens } from '../theme';

const day = 86_400_000;

// Milliseconds until the deadline, or undefined when it is missing or invalid.
function remaining(deadline?: string, now = Date.now()) {
  const time = deadline ? Date.parse(deadline) : NaN;
  return Number.isNaN(time) ? undefined : time - now;
}

// True when the deadline is still ahead and at most `days` away.
export function closesWithin(deadline: string | undefined, days: number, now = Date.now()) {
  const left = remaining(deadline, now);
  return left !== undefined && left > 0 && left <= days * day;
}

export function deadlineLabel(deadline?: string, now = Date.now()) {
  const left = remaining(deadline, now);
  if (left === undefined) return { label: 'No deadline', urgent: false };
  if (left <= 0) return { label: 'Closed', urgent: false };
  if (left <= 7 * day) {
    const days = Math.ceil(left / day);
    return { label: left < day ? 'Closes today' : `Closes in ${days} ${days === 1 ? 'day' : 'days'}`, urgent: true };
  }
  const date = new Date(Date.parse(deadline!));
  const sameYear = date.getFullYear() === new Date(now).getFullYear();
  return { label: `Closes ${date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: sameYear ? undefined : 'numeric' })}`, urgent: false };
}

// "Closes in 2 days" in urgent colours within 7 days, "Closes 9 Dec" otherwise, "Closed" when past.
export function DeadlineTag({ deadline }: { deadline?: string }) {
  const { label, urgent } = deadlineLabel(deadline);
  return <Chip label={label} sx={urgent ? { bgcolor: tokens.urgentBg, color: tokens.urgentText } : undefined} />;
}
