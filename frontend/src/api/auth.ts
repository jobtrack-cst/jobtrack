import { api } from './client';

export const roles = ['ROLE_APPLICANT', 'ROLE_EMPLOYER', 'ROLE_ADMIN'] as const;
export type Role = typeof roles[number];
export interface User { userId: string; email: string; fullName: string; role: Role }
export interface LoginResponse { accessToken: string; expiresAt: string; user: User }
export const login = (email: string, password: string) => api<LoginResponse>('/v1/auth/login', {
  method: 'POST', public: true, body: { email, password },
});
export const register = (email: string, password: string, fullName: string, role: Exclude<Role, 'ROLE_ADMIN'>) =>
  api<{ user: User }>('/v1/auth/register', { method: 'POST', public: true, body: { email, password, fullName, role } });
export const currentUser = () => api<{ user: User }>('/v1/auth/me');
