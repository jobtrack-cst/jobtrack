import { Box, Stack, Typography } from '@mui/material';
import { ListPanel, ListRow } from '../../components/Lists';
import type { HistoryEntry } from './api';
import { formatTime, statusLabel } from './shared';

const realStatus = (status?: string) => !!status && status !== 'APPLICATION_STATUS_UNSPECIFIED' && statusLabel(status) !== 'None';

// Status history, oldest first (JT-49). The first entry has no previous status, so it
// shows only its own status and time, never "from None".
export function StatusHistory({ entries }: { entries: HistoryEntry[] }) {
  return <ListPanel label="Status history">
    {entries.map((e, i) => <ListRow key={i} sx={{ py: 1.75 }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={{ xs: 0.25, sm: 2 }} sx={{ justifyContent: 'space-between' }}>
        <Typography><Box component="strong">{statusLabel(e.newStatus)}</Box>
          {realStatus(e.previousStatus) && <Typography component="span" color="text.secondary">{` from ${statusLabel(e.previousStatus)}`}</Typography>}</Typography>
        <Typography variant="body2" color="text.secondary">{formatTime(e.occurredAt)}</Typography>
      </Stack>
    </ListRow>)}
  </ListPanel>;
}
