import { SvgIcon } from '@mui/material';

const paths: Record<string, string> = {
  jobs: 'M9 6V4h6v2M4 7h16v13H4zM4 12h16M10 12v3h4v-3',
  'saved-jobs': 'M6 3h12v18l-6-4-6 4z',
  notifications: 'M5 17h14l-2-3V9a5 5 0 0 0-10 0v5l-2 3ZM10 21h4',
  applications: 'M8 3h8v4H8zM8 5H5v16h14V5h-3M9 12h6M9 16h6',
  apply: 'M12 4v16M4 12h16',
  offers: 'M4 7h16v13H4zM2 7l10-4 10 4M12 7v13M8 11h8',
  'job-alerts': 'm12 3 3 6 6 1-5 5 1 6-5-3-5 3 1-6-5-5 6-1z',
  profile: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM4 21v-3a8 5 0 0 1 16 0v3',
  resumes: 'M14 3H5v18h14V8zM14 3v5h5M9 12h6M9 16h6',
  company: 'M4 21V7l8-4 8 4v14M2 21h20M8 8h1m6 0h1M8 12h1m6 0h1M10 21v-5h4v5',
  verification: 'm12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6zM8 12l3 3 5-6',
  moderation: 'm12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6zM12 8v5M12 16h.01',
  analytics: 'M4 3v18h17M8 17v-5M13 17V7M18 17v-8',
  'audit-log': 'M6 3h12v18H6zM9 7h6M9 11h6M9 15h6',
  'report-job': 'M5 21V4M5 4h14l-3 5 3 5H5',
};

export function WorkspaceIcon({ name }: { name: string }) {
  return <SvgIcon aria-hidden="true" sx={{ fontSize: 18, flexShrink: 0 }} viewBox="0 0 24 24">
    <path d={paths[name] ?? paths.applications} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
  </SvgIcon>;
}
