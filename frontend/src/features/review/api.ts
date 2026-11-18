import { api } from '../../api/client';
import { idPath } from '../application/shared';

export type ReviewDecision = 'REVIEW_DECISION_SHORTLISTED' | 'REVIEW_DECISION_REJECTED';
export interface Review { reviewId: string; applicationId: string; score: number; decision: ReviewDecision | 'REVIEW_DECISION_UNSPECIFIED'; timestamps?: { createdAt?: string } }

export const getReview = (applicationId: string) => api<{ review: Review }>(`/v1/applications/${idPath(applicationId)}/review`);
export const completeReview = (applicationId: string, score: number, decision: ReviewDecision) =>
  api<{ review: Review }>(`/v1/applications/${idPath(applicationId)}/reviews`, { method: 'POST', body: { score, decision } });
