import { api } from '../../api/client';
import { idPath } from '../application/shared';

export type VerificationStatus = 'VERIFICATION_STATUS_UNSPECIFIED' | 'VERIFICATION_STATUS_UNVERIFIED'
  | 'VERIFICATION_STATUS_VERIFIED' | 'VERIFICATION_STATUS_REJECTED';
export interface CompanyFields { companyName: string; description: string; website: string; location: string }
// protojson leaves out empty strings, so every text field may be missing.
export interface CompanyProfile extends Partial<CompanyFields> {
  employerId: string;
  verificationStatus?: VerificationStatus;
  rejectionReason?: string;
  timestamps?: { createdAt?: string; updatedAt?: string };
  // protojson also leaves out false, so a missing hasLogo means no logo (JT-102).
  hasLogo?: boolean;
}
type ProfileResponse = { profile: CompanyProfile };
// bytes arrive as base64; content_type is the type the service validated.
export interface CompanyLogoResponse { contentType?: string; content?: string }

export const getMyCompanyProfile = () => api<ProfileResponse>('/v1/employer/profile');
export const createCompanyProfile = (fields: CompanyFields) =>
  api<ProfileResponse>('/v1/employer/profile', { method: 'POST', body: fields });
export const updateCompanyProfile = (fields: CompanyFields) =>
  api<ProfileResponse>('/v1/employer/profile', { method: 'PUT', body: fields });
export const getCompanyProfile = (employerId: string) => api<ProfileResponse>(`/v1/companies/${idPath(employerId)}`);
export const putCompanyLogo = (contentType: string, content: string) =>
  api<ProfileResponse>('/v1/employer/profile/logo', { method: 'PUT', body: { contentType, content } });
export const getCompanyLogo = (employerId: string) =>
  api<CompanyLogoResponse>(`/v1/companies/${idPath(employerId)}/logo`);
