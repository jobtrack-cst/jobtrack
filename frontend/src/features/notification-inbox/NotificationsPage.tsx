import { useState } from 'react';
import { Alert, Button, Card, CardContent, Checkbox, FormControlLabel, Stack, Typography } from '@mui/material';
import { Pager, ResourceState } from '../application/shared';
import { formatTime, gatewayMessage } from '../application-status/shared';
import { useLoad } from '../application-status/useLoad';
import { listNotifications, markRead } from './api';

const messages = { 7: 'You can only read your own notifications.' };
export function NotificationsPage() {
  const [token, setToken] = useState('');
  const [unread, setUnread] = useState(false);
  const resource = useLoad(() => listNotifications(token, unread), [token, unread], messages);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function read(id: string) {
    if (busy) return;
    setBusy(true); setError(''); setNotice('');
    try { await markRead(id); setNotice('Notification marked as read.'); setToken(''); resource.refresh(); }
    catch (reason) { setError(gatewayMessage(reason, messages)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Notifications</Typography>
    <FormControlLabel label="Unread only" control={<Checkbox checked={unread} disabled={busy} onChange={(_, value) => { setUnread(value); setToken(''); setError(''); setNotice(''); }} />} />
    <ResourceState {...resource} />{error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    <Button disabled={busy || resource.loading} onClick={() => { setError(''); resource.refresh(); }}>Refresh notifications</Button>
    {resource.data && <Typography>{unread ? 'Unread notifications' : 'Notifications'}: {resource.data.pagination?.totalCount ?? 0}</Typography>}
    {resource.data && !resource.data.notifications?.length && <Typography>No notifications on this page.</Typography>}
    {resource.data?.notifications?.map(n => <Card key={n.notificationId}><CardContent><Stack spacing={1}>
      <Typography variant="h6">{n.title}</Typography><Typography sx={{ whiteSpace: 'pre-wrap' }}>{n.body}</Typography>
      <Typography>{formatTime(n.createdAt)} · {n.read ? 'Read' : 'Unread'}</Typography>
      {!n.read && <Button disabled={busy} onClick={() => void read(n.notificationId)}>Mark as read</Button>}
    </Stack></CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={busy || resource.loading} onPage={setToken} />
  </Stack>;
}
