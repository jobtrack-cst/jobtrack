// "12 Nov", or "12 Nov 2025" outside the current year. Empty when missing or invalid.
export function shortDate(value?: string, now = new Date()) {
  const date = value ? new Date(value) : undefined;
  if (!date || Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: date.getFullYear() === now.getFullYear() ? undefined : 'numeric' });
}

// First word of a full name for greetings; never invents a name.
export const firstName = (fullName?: string) => fullName?.trim().split(/\s+/)[0] ?? '';
