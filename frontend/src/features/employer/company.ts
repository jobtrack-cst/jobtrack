import { length } from '../application-status/shared';
import type { CompanyFields, CompanyProfile } from './api';

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

export function webAddress(value: string) {
  try {
    const url = new URL(value);
    return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname !== '';
  } catch {
    return false;
  }
}

// Save stays disabled until a field differs from the saved profile (JT-109).
export const sameCompany = (a: CompanyFields, b: CompanyFields) =>
  JSON.stringify(trimCompany(a)) === JSON.stringify(trimCompany(b));
