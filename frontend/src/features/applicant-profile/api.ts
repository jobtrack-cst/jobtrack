import { api } from '../../api/client';

export interface Education { institution: string; qualification: string; startYear: number; endYear: number }
export interface ApplicantProfile { applicantId: string; summary?: string; education?: Education[]; skills?: string[] }
export interface ProfileInput { summary: string; education: Education[]; skills: string[] }

export const getMyProfile = () => api<{ profile: ApplicantProfile }>('/v1/applicant/profile');
export const createMyProfile = (input: ProfileInput) => api<{ profile: ApplicantProfile }>('/v1/applicant/profile', { method: 'POST', body: input });
export const updateMyProfile = (input: ProfileInput) => api<{ profile: ApplicantProfile }>('/v1/applicant/profile', { method: 'PUT', body: input });
