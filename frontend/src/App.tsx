import { AppBar, Button, Container, Stack, Toolbar, Typography } from '@mui/material';
import { Link, Navigate, Outlet, Route, Routes } from 'react-router';
import type { Role } from './api/auth';
import { AuthPage } from './features/auth/AuthPage';
import { useAuth } from './features/auth/AuthProvider';
import { landingPath } from './features/auth/routes';
import { ApplicationsPage } from './features/application/ApplicationsPage';
import { ApplyPage } from './features/application/ApplyPage';
import { OffersPage } from './features/offer/OffersPage';
import { MessagesPage } from './features/messaging/MessagesPage';
import { JobAlertsPage } from './features/job-alert/JobAlertsPage';
import { ProfilePage } from './features/applicant-profile/ProfilePage';
import { ProfilePhoto } from './features/applicant-profile/ProfilePhoto';
import { ResumesPage } from './features/resume/ResumesPage';
import { StatusPage } from './features/application-status/StatusPage';
import { ReviewPage } from './features/review/ReviewPage';
import { InterviewsPage } from './features/interview/InterviewsPage';
import { CompanyPage } from './features/employer/CompanyPage';
import { VerificationPage } from './features/company-verification/VerificationPage';
import { AdminVerificationPage } from './features/company-verification/AdminVerificationPage';
import { ReportJobPage } from './features/moderation/ReportJobPage';
import { AdminModerationPage } from './features/moderation/AdminModerationPage';
import { AnalyticsPage } from './features/analytics/AnalyticsPage';
import { AuditLogPage } from './features/audit-log/AuditLogPage';

import { JobsPage } from './features/job-posting/JobsPage';
import { ManageJobPage } from './features/job-posting/ManageJobPage';
import { SearchJobsPage } from './features/job-search/SearchJobsPage';
import { JobDetailsPage } from './features/job-search/JobDetailsPage';
import { SavedJobsPage } from './features/saved-jobs/SavedJobsPage';
import { NotificationsPage } from './features/notification-inbox/NotificationsPage';

// Workspace links per role; each path is relative to the role's landing path.
const navigation: Record<Role, [path: string, label: string][]> = {
  ROLE_APPLICANT: [['jobs', 'Search jobs'], ['saved-jobs', 'Saved jobs'], ['notifications', 'Notifications'], ['applications', 'Applications'], ['apply', 'Apply'], ['offers', 'Offers'], ['job-alerts', 'Job alerts'],
    ['profile', 'Profile'], ['resumes', 'Resumes'], ['report-job', 'Report a job']],
  ROLE_EMPLOYER: [['jobs', 'My jobs'], ['notifications', 'Notifications'], ['applications', 'Applications'], ['offers', 'Offers'], ['company', 'Company'], ['verification', 'Verification'],
    ['report-job', 'Report a job']],
  ROLE_ADMIN: [['notifications', 'Notifications'], ['verification', 'Verification'], ['moderation', 'Moderation'], ['analytics', 'Analytics'], ['audit-log', 'Audit log'],
    ['report-job', 'Report a job']],
};

function Workspace({ role }: { role: Role }) {
  const { session } = useAuth();
  if (!session || session.expiresAt <= Date.now()) return <Navigate to="/login" replace />;
  if (session.user.role !== role) return <Navigate to={landingPath(session.user.role)} replace />;
  const base = landingPath(role);
  return <Stack spacing={3}>
    <Stack component="nav" aria-label="Workspace" direction="row" spacing={1} sx={{ flexWrap: 'wrap' }}>
      {navigation[role].map(([path, label]) => <Button key={path} component={Link} to={`${base}/${path}`}>{label}</Button>)}
    </Stack>
    <Outlet />
  </Stack>;
}
function Home() {
  const { session } = useAuth();
  return <Navigate to={session && session.expiresAt > Date.now() ? landingPath(session.user.role) : '/login'} replace />;
}
export default function App() {
  const { session, signOut } = useAuth();
  return <>
    <AppBar position="static"><Toolbar>
      <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>JobTrack</Typography>
      {/* Applicants see their own photo or initials (JT-103); other roles have no photo. */}
      {session?.user.role === 'ROLE_APPLICANT' && <ProfilePhoto key={session.user.userId} userId={session.user.userId} name={session.user.fullName} size={32} />}
      {session && <Button color="inherit" onClick={signOut} sx={{ ml: 2 }}>Log out</Button>}
    </Toolbar></AppBar>
    <Container component="main" sx={{ py: 4 }}>
      <Routes>
        <Route path="/login" element={<AuthPage key="login" />} />
        <Route path="/register" element={<AuthPage key="register" registration />} />
        <Route path="/applicant" element={<Workspace role="ROLE_APPLICANT" />}>
          <Route index element={<Typography variant="h4" component="h1">Applicant workspace</Typography>} />
          <Route path="jobs" element={<SearchJobsPage />} />
          <Route path="jobs/:jobId" element={<JobDetailsPage />} />
          <Route path="saved-jobs" element={<SavedJobsPage />} />
          <Route path="notifications" element={<NotificationsPage key="applicant-notifications" />} />
          <Route path="apply" element={<ApplyPage />} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="applications/:applicationId/status" element={<StatusPage />} />
          <Route path="applications/:applicationId/interviews" element={<InterviewsPage />} />
          <Route path="offers" element={<OffersPage key="applicant-offers" />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="job-alerts" element={<JobAlertsPage />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="resumes" element={<ResumesPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/applicant" replace />} />
        </Route>
        <Route path="/employer" element={<Workspace role="ROLE_EMPLOYER" />}>
          <Route index element={<Typography variant="h4" component="h1">Employer workspace</Typography>} />
          <Route path="applications" element={<ApplicationsPage />} />
          <Route path="applications/:applicationId/status" element={<StatusPage />} />
          <Route path="applications/:applicationId/review" element={<ReviewPage />} />
          <Route path="applications/:applicationId/interviews" element={<InterviewsPage />} />
          <Route path="offers" element={<OffersPage key="employer-offers" />} />
          <Route path="offers/new/:applicationId" element={<OffersPage key="make-offer" create />} />
          <Route path="messages/:applicationId" element={<MessagesPage />} />
          <Route path="jobs" element={<JobsPage />} />
          <Route path="jobs/new" element={<ManageJobPage key="create-job" create />} />
          <Route path="jobs/:jobId" element={<ManageJobPage key="manage-job" />} />
          <Route path="notifications" element={<NotificationsPage key="employer-notifications" />} />
          <Route path="company" element={<CompanyPage />} />
          <Route path="verification" element={<VerificationPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/employer" replace />} />
        </Route>
        <Route path="/admin" element={<Workspace role="ROLE_ADMIN" />}>
          <Route index element={<Typography variant="h4" component="h1">Admin workspace</Typography>} />
          <Route path="verification" element={<AdminVerificationPage />} />
          <Route path="notifications" element={<NotificationsPage key="admin-notifications" />} />
          <Route path="moderation" element={<AdminModerationPage />} />
          <Route path="analytics" element={<AnalyticsPage />} />
          <Route path="audit-log" element={<AuditLogPage />} />
          <Route path="report-job" element={<ReportJobPage />} />
          <Route path="report-job/:jobId" element={<ReportJobPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Route>
        <Route path="*" element={<Home />} />
      </Routes>
    </Container>
  </>;
}
