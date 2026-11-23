import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { ErrorState } from '../../components/ErrorState';
import { LoadingRows } from '../../components/LoadingRows';
import { TextInput } from '../../components/TextInput';
import { tokens } from '../../theme';
import { pageSize } from '../../components/Pager';
import { errorMessage } from '../application/shared';
import { formatTime } from '../application-status/shared';
import { listMessages, sendMessage, type Message } from './api';

const maxLength = 2000;
const chars = (value: string) => Array.from(value.trim()).length;

interface Props {
  applicationId: string;
  selfId: string;
  // How the other side is named and shown, for example the company name and logo.
  otherName: string;
  otherAvatar: ReactNode;
  // Final applications are view only (redesign doc Section 8).
  readOnly?: boolean;
}

// The thread and composer for one application (redesign doc Section 6). Messages are
// stored oldest first, so the newest page is read using totalCount, and earlier
// pages load on request above it.
export function Conversation({ applicationId, selfId, otherName, otherAvatar, readOnly }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [start, setStart] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [sendError, setSendError] = useState('');
  const thread = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    (async () => {
      const first = await listMessages(applicationId, '');
      const total = Number(first.pagination?.totalCount ?? 0);
      if (total <= pageSize || !first.pagination?.nextPageToken) return { items: first.messages ?? [], start: 0 };
      const offset = total - pageSize;
      const last = await listMessages(applicationId, String(offset));
      return { items: last.messages ?? [], start: offset };
    })().then(r => { if (active) { stickToBottom.current = true; setMessages(r.items); setStart(r.start); } },
      reason => { if (active) setError(errorMessage(reason)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [applicationId, revision]);
  // Newest visible without scrolling, unless earlier messages were just added above.
  useLayoutEffect(() => {
    if (stickToBottom.current && thread.current) thread.current.scrollTop = thread.current.scrollHeight;
  }, [messages]);
  async function earlier() {
    if (loadingEarlier || start === 0) return;
    setLoadingEarlier(true); setError('');
    const offset = Math.max(0, start - pageSize);
    try {
      const page = await listMessages(applicationId, offset ? String(offset) : '');
      // Only the messages before the current first one; the rest are already shown.
      const before = (page.messages ?? []).slice(0, start - offset);
      const known = new Set(messages.map(m => m.messageId));
      stickToBottom.current = false;
      setMessages([...before.filter(m => !known.has(m.messageId)), ...messages]);
      setStart(offset);
    } catch (reason) { setError(errorMessage(reason)); }
    finally { setLoadingEarlier(false); }
  }
  async function send() {
    const content = draft.trim();
    if (busy || readOnly || !content || chars(content) > maxLength) return;
    setBusy(true); setSendError('');
    try {
      const { message } = await sendMessage(applicationId, content);
      setDraft('');
      stickToBottom.current = true;
      if (message?.messageId) setMessages(current => [...current.filter(m => m.messageId !== message.messageId), message]);
      else setRevision(n => n + 1);
    } catch (reason) { setSendError(errorMessage(reason)); }
    finally { setBusy(false); }
  }
  const length = chars(draft);
  return <Stack spacing={2}>
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }}>
      {start > 0 ? <Button disabled={loadingEarlier} onClick={() => void earlier()}>{loadingEarlier ? 'Loading…' : 'Show earlier messages'}</Button> : <span />}
      <Button disabled={loading} onClick={() => setRevision(n => n + 1)}>Refresh messages</Button>
    </Stack>
    {loading && <LoadingRows rows={2} label="Loading messages" />}
    {error && <ErrorState message={error} onRetry={() => setRevision(n => n + 1)} />}
    {!loading && !error && !messages.length && <Typography color="text.secondary">No messages yet.{readOnly ? '' : ` Write to ${otherName} below.`}</Typography>}
    {!!messages.length && <Box ref={thread} role="log" aria-label={`Conversation with ${otherName}`} tabIndex={0}
      sx={{ maxHeight: 480, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 1.5, p: { xs: 1.5, sm: 2 }, borderRadius: '16px',
        bgcolor: 'rgba(14,26,59,0.35)', border: `1px solid ${tokens.border}`, '&:focus-visible': { outline: `3px solid ${tokens.accent}`, outlineOffset: 2 } }}>
      {messages.map(m => { const own = m.senderId === selfId; return <Stack key={m.messageId} direction="row" spacing={1}
        sx={{ alignSelf: own ? 'flex-end' : 'flex-start', alignItems: 'flex-end', maxWidth: { xs: '92%', sm: '75%' }, minWidth: 0 }}>
        {!own && <Box sx={{ flexShrink: 0 }}>{otherAvatar}</Box>}
        <Box sx={{ minWidth: 0, px: 2, py: 1.25, borderRadius: own ? '16px 16px 4px 16px' : '16px 16px 16px 4px',
          ...(own ? { bgcolor: tokens.accent, color: tokens.onAccent } : { bgcolor: tokens.surface, color: tokens.text, border: `1px solid ${tokens.border}` }) }}>
          <Typography component="p" sx={{ fontSize: '0.8125rem', fontWeight: 700, opacity: own ? 0.85 : 1, color: own ? 'inherit' : tokens.textSecondary }}>
            {own ? 'You' : otherName}<Box component="span" sx={{ fontWeight: 500 }}>{m.timestamps?.createdAt ? `, ${formatTime(m.timestamps.createdAt)}` : ''}</Box>
          </Typography>
          <Typography sx={{ mt: 0.5, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{m.content}</Typography>
        </Box>
      </Stack>; })}
    </Box>}
    {readOnly
      ? <Typography variant="body2" color="text.secondary">This application has ended, so the conversation is view only.</Typography>
      : <Stack component="form" spacing={1.5} noValidate onSubmit={e => { e.preventDefault(); void send(); }}>
        <TextInput label="Message" multiline minRows={3} value={draft} disabled={busy} onChange={e => setDraft(e.target.value)}
          error={length > maxLength} helperText={`${length}/${maxLength} characters`} />
        {sendError && <Alert severity="error">{sendError}</Alert>}
        <Button type="submit" variant="contained" disabled={busy || loading || !!error || !length || length > maxLength} sx={{ alignSelf: 'flex-start' }}>
          {busy ? 'Sending…' : 'Send message'}
        </Button>
      </Stack>}
  </Stack>;
}
