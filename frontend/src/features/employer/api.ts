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
}
type ProfileResponse = { profile: CompanyProfile };

export const getMyCompanyProfile = () => api<ProfileResponse>('/v1/employer/profile');
export const createCompanyProfile = (fields: CompanyFields) =>
  api<ProfileResponse>('/v1/employer/profile', { method: 'POST', body: fields });
export const updateCompanyProfile = (fields: CompanyFields) =>
  api<ProfileResponse>('/v1/employer/profile', { method: 'PUT', body: fields });
export const getCompanyProfile = (employerId: string) => api<ProfileResponse>(`/v1/companies/${idPath(employerId)}`);
