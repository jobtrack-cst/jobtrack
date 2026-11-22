import { Button, Stack, Typography } from '@mui/material';

export const pageSize = 20;

interface Props {
  // Current page token: the offset as a decimal string, '' for the first page (AGENTS.md Section 9).
  token: string;
  nextToken?: string;
  totalCount?: number | string;
  // Items shown on this page.
  count: number;
  busy?: boolean;
  onPage: (token: string) => void;
}

// "Showing 1 to 20 of 54" with Previous and Next.
export function Pager({ token, nextToken, totalCount, count, busy, onPage }: Props) {
  const offset = /^\d+$/.test(token) ? Number(token) : 0;
  const total = Number(totalCount ?? 0);
  if (!count && !offset) return null;
  const previous = offset - pageSize > 0 ? String(offset - pageSize) : '';
  return <Stack component="nav" aria-label="Pages" direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap', pt: 1 }}>
    <Typography variant="body2" color="text.secondary" role="status" sx={{ flex: 1, minWidth: 160 }}>
      {count ? `Showing ${offset + 1} to ${offset + count} of ${Math.max(total, offset + count).toLocaleString()}` : 'No more results'}
    </Typography>
    <Button variant="outlined" disabled={busy || offset === 0} onClick={() => onPage(previous)}>Previous</Button>
    <Button variant="outlined" disabled={busy || !nextToken} onClick={() => nextToken && onPage(nextToken)}>Next</Button>
  </Stack>;
}
