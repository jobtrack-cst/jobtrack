import { useState } from 'react';
import { Alert, Button, Chip, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';
import { useParams } from 'react-router';
import { Pager, ResourceState, validId } from '../application/shared';
import { getStatus, listHistory } from './api';
import { formatTime, grpcCode, statusLabel } from './shared';
import { useLoad } from './useLoad';

const readMessages = {
  [grpcCode.notFound]: 'This application has no status yet. It appears shortly after the application is submitted; refresh to try again.',
  [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see its status.',
};

export function StatusPage() {
  const { applicationId = '' } = useParams();
  if (!validId(applicationId)) return <Alert severity="error">The application ID in the address is not valid.</Alert>;
  return <StatusView applicationId={applicationId.toLowerCase()} />;
}

function StatusView({ applicationId }: { applicationId: string }) {
  const [token, setToken] = useState('');
  const status = useLoad(() => getStatus(applicationId), [applicationId], readMessages);
  const history = useLoad(() => listHistory(applicationId, token), [applicationId, token], readMessages);
  const record = status.data?.statusRecord;
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Application status</Typography>
    <Typography variant="body2">Application: {applicationId}</Typography>
    <Button disabled={status.loading || history.loading} onClick={() => { status.refresh(); history.refresh(); }}>Refresh status</Button>
    <ResourceState loading={status.loading} error={status.error} />
    {record && <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      <Typography>Current status:</Typography><Chip label={statusLabel(record.status)} color="primary" />
      <Typography variant="body2">since {formatTime(record.timestamps?.updatedAt)}</Typography>
    </Stack>}
    <Typography variant="h6" component="h2">History</Typography>
    <ResourceState loading={history.loading} error={status.error ? '' : history.error} />
    {history.data && !history.data.entries?.length && <Typography>No history on this page.</Typography>}
    {!!history.data?.entries?.length && <Table size="small" aria-label="Status history"><TableHead><TableRow>
      <TableCell>From</TableCell><TableCell>To</TableCell><TableCell>When</TableCell>
    </TableRow></TableHead><TableBody>
      {history.data.entries.map((e, i) => <TableRow key={`${token}-${i}`}>
        <TableCell>{statusLabel(e.previousStatus)}</TableCell><TableCell>{statusLabel(e.newStatus)}</TableCell><TableCell>{formatTime(e.occurredAt)}</TableCell>
      </TableRow>)}
    </TableBody></Table>}
    <Pager next={history.data?.pagination?.nextPageToken} token={token} busy={history.loading} onPage={setToken} />
  </Stack>;
}
