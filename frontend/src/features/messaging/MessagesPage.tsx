import { useState } from 'react';
import { Alert, Button, Card, CardContent, Stack, TextField, Typography } from '@mui/material';
import { useParams } from 'react-router';
import { useAuth } from '../auth/AuthProvider';
import { errorMessage, Pager, ResourceState, useResource, validId } from '../application/shared';
import { listMessages, sendMessage } from './api';

export function MessagesPage() {
  const { applicationId = '' } = useParams();
  const { session } = useAuth();
  const [token, setToken] = useState('');
  const resource = useResource(() => validId(applicationId) ? listMessages(applicationId, token) : Promise.reject(new Error('Enter a valid application ID.')), [applicationId, token]);
  const [content, setContent] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  async function send() {
    if (busy || !content.trim() || Array.from(content.trim()).length > 2000) return;
    setBusy(true); setError(''); setNotice('');
    try { await sendMessage(applicationId, content.trim()); setContent(''); setNotice('Message sent. Messages are listed oldest first; use Next page to see later messages.'); resource.refresh(); }
    catch (reason) { setError(errorMessage(reason)); }
    finally { setBusy(false); }
  }
  return <Stack spacing={2}>
    <Typography variant="h4" component="h1">Application messages</Typography><Typography>Application: {applicationId}</Typography>
    <Button disabled={busy || resource.loading} onClick={resource.refresh}>Refresh messages</Button><ResourceState {...resource} />
    {resource.data && !resource.data.messages?.length && <Typography>No messages on this page.</Typography>}
    {resource.data?.messages?.map(m => <Card key={m.messageId}><CardContent>
      <Typography>{m.senderId === session?.user.userId ? 'You' : 'Other participant'}{m.timestamps?.createdAt ? ` · ${new Date(m.timestamps.createdAt).toLocaleString()}` : ''}</Typography>
      <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{m.content}</Typography>
    </CardContent></Card>)}
    <Pager token={token} next={resource.data?.pagination?.nextPageToken} busy={resource.loading || busy} onPage={setToken} />
    {error && <Alert severity="error">{error}</Alert>}{notice && <Alert severity="success">{notice}</Alert>}
    <Stack component="form" spacing={2} onSubmit={e => { e.preventDefault(); void send(); }}>
      <TextField label="Message" multiline minRows={3} required value={content} disabled={busy} onChange={e => setContent(e.target.value)} helperText={`${Array.from(content.trim()).length}/2000 characters`} />
      <Button type="submit" disabled={busy || !resource.data || !content.trim() || Array.from(content.trim()).length > 2000}>Send message</Button>
    </Stack>
  </Stack>;
}
