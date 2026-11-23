import { Box, Tab, Tabs as MuiTabs } from '@mui/material';
import type { ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { tokens } from '../theme';

// The selected tab lives in ?tab=, so Back, reloads and links (for example
// /applicant/profile?tab=resumes) open the same tab. The first tab is the default.
export function useTab<T extends string>(values: readonly T[]): [T, (value: T) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get('tab');
  const current = values.find(v => v === raw) ?? values[0];
  const select = (value: T) => setParams(previous => {
    const next = new URLSearchParams(previous);
    if (value === values[0]) next.delete('tab'); else next.set('tab', value);
    return next;
  }, { replace: true });
  return [current, select];
}

const tabId = (base: string, value: string) => `${base}-tab-${value}`;
const panelId = (base: string) => `${base}-panel`;

// A row of tabs over one panel. Labels are plain words; counts go in the label when known.
export function TabBar<T extends string>({ id, label, value, tabs, onChange }: {
  id: string; label: string; value: T; tabs: [value: T, label: string][]; onChange: (value: T) => void;
}) {
  return <Box sx={{ borderBottom: `1px solid ${tokens.border}` }}>
    <MuiTabs value={value} onChange={(_, next: T) => onChange(next)} aria-label={label} variant="scrollable" scrollButtons={false}>
      {tabs.map(([v, text]) => <Tab key={v} value={v} label={text} id={tabId(id, v)} aria-controls={panelId(id)} />)}
    </MuiTabs>
  </Box>;
}

export function TabPanel({ id, value, children }: { id: string; value: string; children: ReactNode }) {
  return <Box role="tabpanel" id={panelId(id)} aria-labelledby={tabId(id, value)} sx={{ minWidth: 0 }}>{children}</Box>;
}
