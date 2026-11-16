import { length } from '../application-status/shared';
import type { CompanyFields, CompanyProfile, VerificationStatus } from './api';

export const emptyCompany: CompanyFields = { companyName: '', description: '', website: '', location: '' };

export const fromProfile = (p: CompanyProfile): CompanyFields => ({
  companyName: p.companyName ?? '', description: p.description ?? '', website: p.website ?? '', location: p.location ?? '',
});

export const trimCompany = (f: CompanyFields): CompanyFields => ({
  companyName: f.companyName.trim(), description: f.description.trim(), website: f.website.trim(), location: f.location.trim(),
});

// JT-32 limits, counted after trimming as the service does. An empty company
// name is not listed here; the form keeps Save disabled instead.
export function companyProblems(f: CompanyFields) {
  const problems: string[] = [];
  if (length(f.companyName) > 150) problems.push('Company name can have at most 150 characters.');
  if (length(f.description) > 2000) problems.push('Description can have at most 2000 characters.');
  const website = f.website.trim();
  if (length(website) > 255) problems.push('Website can have at most 255 characters.');
  else if (website && !webAddress(website)) problems.push('Website must be a full address starting with http:// or https://, for example https://example.bt.');
  if (length(f.location) > 100) problems.push('Location can have at most 100 characters.');
  return problems;
}

function webAddress(value: string) {
  try {
    const url = new URL(value);
    return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname !== '';
  } catch {
    return false;
  }
}

type Color = 'default' | 'success' | 'error';
const views: Record<VerificationStatus, { label: string; color: Color }> = {
  VERIFICATION_STATUS_UNSPECIFIED: { label: 'Unknown', color: 'default' },
  VERIFICATION_STATUS_UNVERIFIED: { label: 'Not verified', color: 'default' },
  VERIFICATION_STATUS_VERIFIED: { label: 'Verified', color: 'success' },
  VERIFICATION_STATUS_REJECTED: { label: 'Rejected', color: 'error' },
};
export const verificationOf = (status?: VerificationStatus) =>
  views[status ?? 'VERIFICATION_STATUS_UNSPECIFIED'] ?? views.VERIFICATION_STATUS_UNSPECIFIED;
