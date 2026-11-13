import type { Role } from '../../api/auth';
export const landingPath = (role: Role) => ({ ROLE_APPLICANT: '/applicant', ROLE_EMPLOYER: '/employer', ROLE_ADMIN: '/admin' })[role];
