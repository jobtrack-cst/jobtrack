import { SvgIcon, type SxProps, type Theme } from '@mui/material';

// Stroke icons on a 24px grid. Inline, because no icon package is installed.
const paths = {
  home: 'M4 11 12 4l8 7M6 9.5V20h12V9.5M10 20v-5h4v5',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14ZM20 20l-4-4',
  bookmark: 'M6 3h12v18l-6-4-6 4z',
  applications: 'M8 3h8v4H8zM8 5H5v16h14V5h-3M9 12h6M9 16h6',
  messages: 'M4 5h16v11H9l-5 4z',
  profile: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM4 21v-2a8 5 0 0 1 16 0v2',
  alerts: 'M5 17h14l-2-3V9a5 5 0 0 0-10 0v5l-2 3ZM10 21h4',
  jobs: 'M9 6V4h6v2M4 7h16v13H4zM4 12h16M10 12v3h4v-3',
  people: 'M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM2 21v-1a7 5 0 0 1 14 0v1M16 3.5a4 4 0 0 1 0 7.5M18 15.5a6 5 0 0 1 4 4.5v1',
  company: 'M4 21V7l8-4 8 4v14M2 21h20M8 8h1m6 0h1M8 12h1m6 0h1M10 21v-5h4v5',
  verification: 'm12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6zM8 12l3 3 5-6',
  moderation: 'M5 21V4M5 4h14l-3 5 3 5H5',
  analytics: 'M4 3v18h17M8 17v-5M13 17V7M18 17v-8',
  audit: 'M6 3h12v18H6zM9 7h6M9 11h6M9 15h4',
  menu: 'M4 6h16M4 12h16M4 18h16',
  close: 'm6 6 12 12M6 18 18 6',
  chevronDown: 'm6 9 6 6 6-6',
  chevronRight: 'm9 6 6 6-6 6',
  chevronLeft: 'm15 6-6 6 6 6',
  logout: 'M9 4H4v16h5m5-13 5 5-5 5m-6-5h11',
  check: 'm5 12 5 5 9-10',
  clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18ZM12 7v5l3 2',
  plus: 'M12 5v14M5 12h14',
  refresh: 'M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5',
  warning: 'M12 3 2 20h20zM12 10v4M12 17h.01',
} as const;

export type IconName = keyof typeof paths;

export function Icon({ name, size = 20, sx }: { name: IconName; size?: number; sx?: SxProps<Theme> }) {
  return <SvgIcon aria-hidden="true" viewBox="0 0 24 24" sx={[{ fontSize: size, flexShrink: 0 }, ...(Array.isArray(sx) ? sx : [sx])]}>
    <path d={paths[name]} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
  </SvgIcon>;
}
