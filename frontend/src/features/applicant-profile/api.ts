import { api } from '../../api/client';

export interface Education { institution: string; qualification: string; startYear: number; endYear: number }
// protojson leaves out false, so a missing hasPhoto means no photo (JT-103).
export interface ApplicantProfile { applicantId: string; summary?: string; education?: Education[]; skills?: string[]; hasPhoto?: boolean }
// bytes arrive as base64; content_type is the type the service validated.
export interface ProfilePhotoResponse { contentType?: string; content?: string }
export interface ProfileInput { summary: string; education: Education[]; skills: string[] }

export const getMyProfile = () => api<{ profile: ApplicantProfile }>('/v1/applicant/profile');
export const createMyProfile = (input: ProfileInput) => api<{ profile: ApplicantProfile }>('/v1/applicant/profile', { method: 'POST', body: input });
export const updateMyProfile = (input: ProfileInput) => api<{ profile: ApplicantProfile }>('/v1/applicant/profile', { method: 'PUT', body: input });
export const putMyProfilePhoto = (contentType: string, content: string) =>
  api<{ profile: ApplicantProfile }>('/v1/applicant/profile/photo', { method: 'PUT', body: { contentType, content } });
export const getMyProfilePhoto = () => api<ProfilePhotoResponse>('/v1/applicant/profile/photo');
