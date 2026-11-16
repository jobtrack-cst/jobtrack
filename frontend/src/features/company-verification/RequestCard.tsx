import type { ReactNode } from 'react';
import { Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { formatTime } from '../application-status/shared';
import type { RequestStatus, VerificationRequest } from './api';

type Color = 'default' | 'warning' | 'success' | 'error';
const views: Record<RequestStatus, { label: string; color: Color }> = {
  REQUEST_STATUS_UNSPECIFIED: { label: 'Unknown', color: 'default' },
  REQUEST_STATUS_PENDING: { label: 'Pending', color: 'warning' },
  REQUEST_STATUS_APPROVED: { label: 'Approved', color: 'success' },
  REQUEST_STATUS_REJECTED: { label: 'Rejected', color: 'error' },
};

// One verification request. Admins also see who decided it; actions come in as children.
export function RequestCard({ request, admin, children }: { request: VerificationRequest; admin?: boolean; children?: ReactNode }) {
  const view = views[request.status ?? 'REQUEST_STATUS_UNSPECIFIED'] ?? views.REQUEST_STATUS_UNSPECIFIED;
  const decided = request.status === 'REQUEST_STATUS_APPROVED' || request.status === 'REQUEST_STATUS_REJECTED';
  return <Card><CardContent><Stack spacing={1}>
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
      <Typography variant="h6" component="h3">Registration number: {request.registrationNumber || 'Not set'}</Typography>
      <Chip label={view.label} color={view.color} />
    </Stack>
    {request.notes && <Typography sx={{ whiteSpace: 'pre-wrap' }}>Notes: {request.notes}</Typography>}
    <Typography>Submitted: {formatTime(request.timestamps?.createdAt)}</Typography>
    {request.status === 'REQUEST_STATUS_REJECTED' && <Typography>Reason: {request.rejectionReason || 'Not given'}</Typography>}
    {decided && <Typography variant="body2">
      Decided: {formatTime(request.timestamps?.updatedAt)}{admin && request.reviewedBy ? ` by admin ${request.reviewedBy}` : ''}
    </Typography>}
    <Typography variant="body2">Request ID: {request.requestId}</Typography>
    {children}
  </Stack></CardContent></Card>;
}
