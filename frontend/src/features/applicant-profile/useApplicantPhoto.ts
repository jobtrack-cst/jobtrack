import { useEffect, useState } from 'react';
import { base64ToBlob } from '../../components/blob';
import { getApplicantPhotoForApplication } from './api';

const photoTypes = new Set(['image/jpeg', 'image/png']);

// The applicant's photo for one application, for its owning employer (JT-114 Part C).
// Read on every mount with no cache, as an object URL that is revoked on unmount or
// when the application changes. NOT_FOUND (no photo, no profile, or an application
// from before JT-114) and every other failure give null, so initials are shown.
export function useApplicantPhoto(applicationId: string): string | null {
  const [photo, setPhoto] = useState<{ id: string; url: string } | null>(null);
  useEffect(() => {
    let active = true;
    let url = '';
    getApplicantPhotoForApplication(applicationId).then(({ contentType = '', content }) => {
      // Only validated image types become an image address.
      if (!active || !content || !photoTypes.has(contentType)) return;
      url = URL.createObjectURL(base64ToBlob(content, contentType));
      setPhoto({ id: applicationId, url });
    }, () => { /* Initials are shown instead. */ });
    return () => {
      active = false;
      if (url) URL.revokeObjectURL(url);
      setPhoto(null);
    };
  }, [applicationId]);
  return photo?.id === applicationId ? photo.url : null;
}
