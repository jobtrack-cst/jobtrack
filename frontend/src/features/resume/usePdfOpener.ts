import { useEffect, useState } from 'react';
import { base64ToBlob } from '../../components/blob';
import { gatewayMessage, isNotFound } from '../application-status/shared';
import type { ResumeFile } from './api';

// How long a file opened in a new tab keeps its address; the tab has loaded it by then.
const keepOpenedMs = 60_000;

// Opens a stored resume as a PDF in a new tab (JT-112 View PDF, JT-111 the employer's
// Download resume). The tab is opened during the click, so popup blockers allow it; if the
// browser blocks it anyway, `fallback` holds an address for a Download PDF link, revoked
// on unmount or when replaced.
// With `notFoundIsEmpty`, NOT_FOUND sets `missing` (an empty state, for example an application
// from before JT-114 that has no stored resume) instead of an error.
export function usePdfOpener(messages: Partial<Record<number, string>>, { notFoundIsEmpty = false } = {}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [missing, setMissing] = useState(false);
  const [fallback, setFallback] = useState<{ url: string; fileName: string }>();
  useEffect(() => () => { if (fallback) URL.revokeObjectURL(fallback.url); }, [fallback]);
  async function open(load: () => Promise<ResumeFile>, title: string) {
    if (busy) return;
    setError(''); setMissing(false);
    const tab = window.open('', '_blank');
    if (tab) { tab.opener = null; tab.document.title = title; }
    setBusy(true);
    try {
      const file = await load();
      if (!file.content) throw new Error('The file could not be read. Please try again.');
      // Always a PDF: only validated PDFs are stored (JT-48), and nothing else may open from here.
      const url = URL.createObjectURL(base64ToBlob(file.content, 'application/pdf'));
      if (tab && !tab.closed) {
        tab.location.href = url;
        window.setTimeout(() => URL.revokeObjectURL(url), keepOpenedMs);
      } else setFallback({ url, fileName: file.fileName || title });
    } catch (reason) {
      tab?.close();
      if (notFoundIsEmpty && isNotFound(reason)) setMissing(true);
      else setError(gatewayMessage(reason, messages));
    } finally { setBusy(false); }
  }
  return { busy, error, missing, fallback, open };
}
