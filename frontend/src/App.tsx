import { Box, LinearProgress, Typography } from '@mui/material';
import { Navigate, Outlet, Route, Routes, useParams, useSearchParams } from 'react-router';
import { validId } from './features/application/shared';
import type { Role } from './api/auth';
import { AppShell } from './components/AppShell';
import { AdminHome } from './features/home/AdminHome';
import { ApplicantHome } from './features/home/ApplicantHome';
import { EmployerHome } from './features/home/EmployerHome';
import { AuthPage } from './features/auth/AuthPage';
import { useAuth } from './features/auth/AuthProvider';
import { landingPath } from './features/auth/routes';
import { ApplicantsPage } from './features/application/ApplicantsPage';
import { MyApplicationsPage } from './features/application/MyApplicationsPage';
import { ApplicationPage } from './features/application/ApplicationPage';
import { EmployerApplicationPage } from './features/application/EmployerApplicationPage';
import { JobAlertsPage } from './features/job-alert/JobAlertsPage';
import { ProfilePage } from './features/applicant-profile/ProfilePage';
import { CompanyPage } from './features/employer/CompanyPage';
import { AdminVerificationPage } from './features/company-verification/AdminVerificationPage';
import { ReportJobPage } from './features/moderation/ReportJobPage';
import { AdminModerationPage } from './features/moderation/AdminModerationPage';
import { AnalyticsPage } from './features/analytics/AnalyticsPage';
import { AuditLogPage } from './features/audit-log/AuditLogPage';
import { JobsPage } from './features/job-posting/JobsPage';
import { JobPage } from './features/job-posting/JobPage';
import { SearchJobsPage } from './features/job-search/SearchJobsPage';
import { JobDetailsPage } from './features/job-search/JobDetailsPage';
import { SavedJobsPage } from './features/saved-jobs/SavedJobsPage';
import { NotificationsPage } from './features/notification-inbox/NotificationsPage';
import { MessagesPage } from './features/messaging/MessagesPage';

// Only the signed-in role may see its workspace; anyone else is sent to their own.
function Workspace({ role }: { role: Role }) {
  const { session } = useAuth();
  if (!session || session.expiresAt <= Date.now()) return <Navigate to="/login" replace />;
  if (session.user.role !== role) return <Navigate to={landingPath(session.user.role)} replace />;
  return <Box component="section" aria-label="Page content" className="workspace-content" sx={{ minWidth: 0, overflowWrap: 'anywhere' }}><Outlet /></Box>;
}

function Home() {
  const { session } = useAuth();
  return <Navigate to={session && session.expiresAt > Date.now() ? landingPath(session.user.role) : '/login'} replace />;
}

// Old addresses with an ID (path or query) go to the page that replaced them;
// without a valid ID they go to the list (JT-112 applicant, JT-111 employer).
function ToJob({ query }: { query?: string }) {
  const { jobId = '' } = useParams();
  const [params] = useSearchParams();
  const id = query ? params.get(query) ?? '' : jobId;
  return <Navigate to={validId(id) ? `/applicant/jobs/${encodeURIComponent(id.toLowerCase())}` : '/applicant/jobs'} replace />;
}

function ToApplication({ query, list = '/applicant/applications' }: { query?: string; list?: string }) {
  const { applicationId = '' } = useParams();
  const [params] = useSearchParams();
  const id = query ? params.get(query) ?? '' : applicationId;
  return <Navigate to={validId(id) ? `${list}/${encodeURIComponent(id.toLowerCase())}` : list} replace />;
}
const employerList = '/employer/applicants';

export default function App() {
  const { session, initializing, signOut } = useAuth();
  const user = session && session.expiresAt > Date.now() ? session.user : undefined;
  if (initializing) return <Box component="main" sx={{ maxWidth: 480, mx: 'auto', px: 2, py: 8 }}>
    <LinearProgress sx={{ mb: 3 }} /><Typography role="status">Restoring session…</Typography>
  </Box>;
  return <AppShell user={user} onSignOut={signOut}>
    <Routes>
      <Route path="/login" element={<AuthPage key="login" />} />
      <Route path="/register" element={<AuthPage key="register" registration />} />
      <Route path="/applicant" element={<Workspace role="ROLE_APPLICANT" />}>
        <Route index element={<ApplicantHome />} />
        <Route path="jobs" element={<SearchJobsPage />} />
        <Route path="jobs/:jobId" element={<JobDetailsPage />} />
        <Route path="saved" element={<SavedJobsPage />} />
        <Route path="notifications" element={<NotificationsPage key="applicant-notifications" />} />
        <Route path="applications" element={<MyApplicationsPage />} />
        <Route path="applications/:applicationId" element={<ApplicationPage />} />
        {/* JT-113 inbox; the older messages/:applicationId address now opens its thread here. */}
        <Route path="messages/:applicationId?" element={<MessagesPage />} />
        <Route path="job-alerts" element={<JobAlertsPage />} />
        <Route path="profile" element={<ProfilePage />} />
        {/* JT-112: addresses of the pages that the redesign merged into others. */}
        <Route path="saved-jobs" element={<Navigate to="/applicant/saved" replace />} />
        <Route path="resumes" element={<Navigate to="/applicant/profile?tab=resumes" replace />} />
        <Route path="apply" element={<ToJob query="jobId" />} />
        <Route path="report-job" element={<Navigate to="/applicant/jobs" replace />} />
        <Route path="report-job/:jobId" element={<ToJob />} />
        <Route path="offers" element={<ToApplication query="applicationId" />} />
        <Route path="applications/:applicationId/status" element={<ToApplication />} />
        <Route path="applications/:applicationId/interviews" element={<ToApplication />} />
        <Route path="*" element={<Navigate to="/applicant" replace />} />
      </Route>
      <Route path="/employer" element={<Workspace role="ROLE_EMPLOYER" />}>
        <Route index element={<EmployerHome />} />
        <Route path="applicants" element={<ApplicantsPage />} />
        <Route path="applicants/:applicationId" element={<EmployerApplicationPage />} />
        <Route path="messages/:applicationId?" element={<MessagesPage />} />
        <Route path="jobs" element={<JobsPage />} />
        <Route path="jobs/new" element={<JobPage key="create-job" create />} />
        <Route path="jobs/:jobId" element={<JobPage key="manage-job" />} />
        <Route path="notifications" element={<NotificationsPage key="employer-notifications" />} />
        <Route path="company" element={<CompanyPage />} />
        {/* JT-111: addresses of the pages that the redesign merged into others. Employers
            only manage their own jobs, so they have no Report a job page. */}
        <Route path="verification" element={<Navigate to="/employer/company" replace />} />
        <Route path="applications" element={<Navigate to={employerList} replace />} />
        <Route path="applications/:applicationId/status" element={<ToApplication list={employerList} />} />
        <Route path="applications/:applicationId/review" element={<ToApplication list={employerList} />} />
        <Route path="applications/:applicationId/interviews" element={<ToApplication list={employerList} />} />
        <Route path="offers" element={<ToApplication query="applicationId" list={employerList} />} />
        <Route path="offers/new/:applicationId" element={<ToApplication list={employerList} />} />
        <Route path="report-job" element={<Navigate to="/employer/jobs" replace />} />
        <Route path="report-job/:jobId" element={<Navigate to="/employer/jobs" replace />} />
        <Route path="*" element={<Navigate to="/employer" replace />} />
      </Route>
      <Route path="/admin" element={<Workspace role="ROLE_ADMIN" />}>
        <Route index element={<AdminHome />} />
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
  </AppShell>;
}
