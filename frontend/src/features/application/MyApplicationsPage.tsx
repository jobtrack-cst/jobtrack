import { useState } from 'react';
import { Button, Stack } from '@mui/material';
import { Link } from 'react-router';
import { LogoSquare, useCompany } from '../../components/CompanyAvatar';
import { EmptyState } from '../../components/EmptyState';
import { ErrorState } from '../../components/ErrorState';
import { CardGrid } from '../../components/Lists';
import { LoadingRows } from '../../components/LoadingRows';
import { MiniCard } from '../../components/MiniCard';
import { PageHeader } from '../../components/PageHeader';
import { Pager, pageSize } from '../../components/Pager';
import { StatusChip } from '../../components/StatusChip';
import { TabBar, TabPanel, useTab } from '../../components/Tabs';
import { jobName, useJob } from '../../components/useJob';
import { useLoad } from '../application-status/useLoad';
import { loadMyApplications, type Tracked } from './applicant';

const tabs = ['active', 'past'] as const;

function ApplicationCard({ item }: { item: Tracked }) {
  const job = useJob(item.app.jobId);
  const { company } = useCompany(item.app.employerId);
  const line = [company?.name, job.job?.location].filter(Boolean).join(', ');
  return <MiniCard avatar={<LogoSquare src={company?.url} name={company?.name} size={48} />} title={jobName(job)} line={line || undefined}
    chip={<StatusChip kind="application" status={item.status} />} viewTo={`/applicant/applications/${encodeURIComponent(item.app.applicationId)}`} />;
}

// My applications: one card per job (its newest application). Withdrawn and final
// ones are under Past. Every page is read first, so the tabs and counts are exact;
// the Pager then pages through the cards here.
export function MyApplicationsPage() {
  const [tab, setTab] = useTab(tabs);
  const resource = useLoad(loadMyApplications, [], {});
  const [token, setToken] = useState('');
  const all = resource.data ?? [];
  const active = all.filter(x => x.active);
  const past = all.filter(x => !x.active);
  const list = tab === 'active' ? active : past;
  const offset = Number(token) || 0;
  const page = list.slice(offset, offset + pageSize);
  const next = offset + pageSize < list.length ? String(offset + pageSize) : undefined;
  const count = (items: Tracked[]) => resource.data ? ` (${items.length})` : '';
  return <Stack spacing={4}>
    <PageHeader title="Applications" description="Follow each application from submitted to offer."
      action={<Button component={Link} to="/applicant/jobs" variant="contained">Find jobs</Button>} />
    <Stack spacing={3}>
      <TabBar id="applications" label="Applications" value={tab} onChange={value => { setTab(value); setToken(''); }}
        tabs={[['active', `Active${count(active)}`], ['past', `Past${count(past)}`]]} />
      <TabPanel id="applications" value={tab}>
        <Stack spacing={2}>
          {resource.loading && <LoadingRows rows={3} height={140} label="Loading applications" />}
          {resource.error && <ErrorState message={resource.error} onRetry={resource.refresh} />}
          {resource.data && !list.length && (tab === 'active'
            ? <EmptyState title="No active applications" description="When you apply for a job, it shows here until it ends."
              action={<Button component={Link} to="/applicant/jobs" variant="outlined">Find jobs</Button>} />
            : <EmptyState title="No past applications" description="Withdrawn, accepted, declined and rejected applications show here." />)}
          {!!page.length && <CardGrid label={tab === 'active' ? 'Active applications' : 'Past applications'}>
            {page.map(item => <ApplicationCard key={item.app.applicationId} item={item} />)}
          </CardGrid>}
          {resource.data && <Pager token={token} nextToken={next} totalCount={list.length} count={page.length} onPage={setToken} />}
        </Stack>
      </TabPanel>
    </Stack>
  </Stack>;
}
