import { Chip } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { tokens } from '../theme';

export type Tone = 'neutral' | 'accent' | 'success' | 'danger';
type View = [label: string, tone: Tone];

// Plain labels for every status enum the UI shows; enum names never reach the screen.
const views = {
  application: {
    APPLICATION_STATUS_SUBMITTED: ['Submitted', 'neutral'], APPLICATION_STATUS_UNDER_REVIEW: ['Under review', 'neutral'],
    APPLICATION_STATUS_SHORTLISTED: ['Shortlisted', 'accent'], APPLICATION_STATUS_INTERVIEW: ['Interview', 'accent'],
    APPLICATION_STATUS_OFFERED: ['Offered', 'success'], APPLICATION_STATUS_ACCEPTED: ['Accepted', 'success'],
    APPLICATION_STATUS_DECLINED: ['Declined', 'danger'], APPLICATION_STATUS_REJECTED: ['Rejected', 'danger'],
    APPLICATION_STATUS_WITHDRAWN: ['Withdrawn', 'danger'],
  },
  job: { JOB_STATUS_DRAFT: ['Draft', 'neutral'], JOB_STATUS_PUBLISHED: ['Published', 'success'], JOB_STATUS_CLOSED: ['Closed', 'neutral'], JOB_STATUS_REMOVED: ['Removed', 'danger'] },
  resume: { RESUME_VALIDATION_STATUS_PENDING: ['Checking', 'neutral'], RESUME_VALIDATION_STATUS_VALIDATED: ['Validated', 'success'], RESUME_VALIDATION_STATUS_REJECTED: ['Rejected', 'danger'] },
  verification: { VERIFICATION_STATUS_UNVERIFIED: ['Not verified', 'neutral'], VERIFICATION_STATUS_VERIFIED: ['Verified', 'success'], VERIFICATION_STATUS_REJECTED: ['Rejected', 'danger'] },
  request: { REQUEST_STATUS_PENDING: ['Pending', 'accent'], REQUEST_STATUS_APPROVED: ['Approved', 'success'], REQUEST_STATUS_REJECTED: ['Rejected', 'danger'] },
  offer: { OFFER_STATUS_PENDING: ['Awaiting reply', 'accent'], OFFER_STATUS_ACCEPTED: ['Accepted', 'success'], OFFER_STATUS_DECLINED: ['Declined', 'danger'] },
  report: { REPORT_STATUS_OPEN: ['Open', 'accent'], REPORT_STATUS_DISMISSED: ['Dismissed', 'neutral'], REPORT_STATUS_ACTIONED: ['Job removed', 'danger'] },
} satisfies Record<string, Record<string, View>>;

export type StatusKind = keyof typeof views;

export function statusView(kind: StatusKind, status?: string): View {
  return (views[kind] as Record<string, View>)[status ?? ''] ?? ['Unknown', 'neutral'];
}

// Tone colours on surface: each text colour passes 4.5:1 on its tinted background.
export const toneSx: Record<Tone, { bgcolor: string; color: string }> = {
  neutral: { bgcolor: tokens.chipBg, color: tokens.chipText },
  accent: { bgcolor: alpha(tokens.accent, 0.2), color: tokens.accentHover },
  success: { bgcolor: alpha(tokens.success, 0.16), color: tokens.success },
  danger: { bgcolor: alpha(tokens.danger, 0.16), color: tokens.danger },
};

export function ToneChip({ label, tone }: { label: string; tone: Tone }) {
  return <Chip label={label} sx={toneSx[tone]} />;
}

export function StatusChip({ kind, status }: { kind: StatusKind; status?: string }) {
  const [label, tone] = statusView(kind, status);
  return <ToneChip label={label} tone={tone} />;
}
