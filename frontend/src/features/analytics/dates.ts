const pad = (n: number) => String(n).padStart(2, '0');

// Value for a date input, in the browser's time zone.
export const dayInput = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

// Local midnight of a date input value, plus some days. new Date('YYYY-MM-DD')
// would read the value as UTC midnight instead.
export function dayStart(value: string, addDays = 0) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return undefined;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]) + addDays);
  return Number.isNaN(date.getTime()) ? undefined : date;
}
