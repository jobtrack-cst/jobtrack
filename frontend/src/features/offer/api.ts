import { api } from '../../api/client';
import { idPath } from '../application/shared';
export interface Offer { offerId: string; applicationId: string; terms: string; status: string }
export const getOffer = (id: string) => api<{ offer: Offer }>(`/v1/offers/${idPath(id)}`);
export const makeOffer = (id: string, terms: string) => api<{ offer: Offer }>(`/v1/applications/${idPath(id)}/offers`, { method: 'POST', body: { terms } });
export const respondOffer = (id: string, decision: 'accept' | 'decline') => api<{ offer: Offer }>(`/v1/offers/${idPath(id)}/${decision}`, { method: 'POST', body: {} });

export const getOfferByApplication = (id: string) => api<{ offer: Offer }>(`/v1/applications/${idPath(id)}/offer`);
