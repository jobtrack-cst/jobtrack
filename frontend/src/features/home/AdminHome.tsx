import { Box, Stack, Typography } from '@mui/material';
import { Card } from '../../components/Card';
import { Icon, type IconName } from '../../components/Icon';
import { PageHeader } from '../../components/PageHeader';
import { tokens } from '../../theme';
import { useLoad } from '../application-status/useLoad';
import { listVerificationRequests } from '../company-verification/api';
import { listReports } from '../moderation/api';

function CountTile({ to, label, line, load }: { to: string; label: string; line: string; load: () => Promise<{ pagination?: { totalCount?: number | string } }> }) {
  const count = useLoad(load, [], {});
  const value = count.loading ? '…' : count.error ? '–' : Number(count.data?.pagination?.totalCount ?? 0).toLocaleString();
  const spoken = count.loading ? 'loading' : count.error ? 'could not be loaded' : value;
  return <Card to={to} label={`${label}: ${spoken}. ${line}`}>
    <Typography color="text.secondary" sx={{ fontWeight: 600 }}>{label}</Typography>
    <Typography sx={{ fontSize: '2.5rem', fontWeight: 800, letterSpacing: '-0.04em', lineHeight: 1.2, mt: 0.5, fontVariantNumeric: 'tabular-nums' }}>{value}</Typography>
    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>{count.error ? 'Could not be loaded. Open the page to try again.' : line}</Typography>
  </Card>;
}

const links: [to: string, title: string, line: string, icon: IconName][] = [
  ['/admin/verification', 'Verification', 'Approve or reject employer registrations.', 'verification'],
  ['/admin/moderation', 'Moderation', 'Review job reports and remove jobs.', 'moderation'],
  ['/admin/analytics', 'Analytics', 'Count hiring events for a date range.', 'analytics'],
  ['/admin/audit-log', 'Audit log', 'Every domain event, newest first.', 'audit'],
];

export function AdminHome() {
  return <Stack spacing={4}>
    <PageHeader title="Home" description="What needs your attention across JobTrack." />
    <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: 'minmax(0, 1fr)', sm: 'repeat(2, minmax(0, 1fr))' } }}>
      <CountTile to="/admin/verification" label="Pending verification" line="Employers waiting for a decision."
        load={() => listVerificationRequests('', 'REQUEST_STATUS_PENDING')} />
      <CountTile to="/admin/moderation" label="Open reports" line="Job reports waiting for review."
        load={() => listReports('', 'REPORT_STATUS_OPEN')} />
    </Box>
    <Stack component="section" spacing={2} aria-label="Admin pages">
      <Typography variant="h2">Admin pages</Typography>
      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0, display: 'grid', gap: 1.5, gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'repeat(2, minmax(0, 1fr))' } }}>
        {links.map(([to, title, line, icon]) => <Card key={to} to={to} component="li">
          <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
            <Box sx={{ width: 44, height: 44, borderRadius: '12px', display: 'grid', placeItems: 'center', bgcolor: tokens.chipBg, color: tokens.chipText, flexShrink: 0 }}>
              <Icon name={icon} />
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="h3" component="h3">{title}</Typography>
              <Typography variant="body2" color="text.secondary">{line}</Typography>
            </Box>
          </Stack>
        </Card>)}
      </Box>
    </Stack>
  </Stack>;
}
