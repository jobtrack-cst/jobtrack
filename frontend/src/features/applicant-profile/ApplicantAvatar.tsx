import { LogoSquare } from '../../components/CompanyAvatar';
import { useApplicantPhoto } from './useApplicantPhoto';

// An applicant's photo circle for the owning employer, or the initials of the name shown
// beside it ("A" for "Applicant"). Decorative: the name is always shown as text.
export function ApplicantAvatar({ applicationId, name, size = 48 }: { applicationId: string; name: string; size?: number }) {
  const url = useApplicantPhoto(applicationId);
  return <LogoSquare src={url} name={name} size={size} />;
}
