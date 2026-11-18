import { useEffect, useState } from 'react';
import { gatewayMessage } from './shared';

// Like useResource from the application feature, but maps errors with this
// page's messages instead of the application and offer wording.
export function useLoad<T>(load: () => Promise<T>, dependencies: unknown[], messages: Partial<Record<number, string>>) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState('');
  const [failure, setFailure] = useState<unknown>();
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true); setError(''); setFailure(undefined); setData(undefined);
    load().then(value => { if (active) setData(value); }, reason => { if (active) { setFailure(reason); setError(gatewayMessage(reason, messages)); } })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
    // Dependencies are supplied by each caller.
  }, [...dependencies, revision]);
  return { data, error, failure, loading, refresh: () => setRevision(n => n + 1) };
}
