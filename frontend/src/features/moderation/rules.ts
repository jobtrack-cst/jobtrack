import { validId } from '../application/shared';
import { length } from '../application-status/shared';

// JT-34 limits, counted after trimming as the service does. Empty values are
// not listed; the forms keep their buttons disabled instead.
export function reasonProblems(jobId: string, reason: string, maxReason: number) {
  const problems: string[] = [];
  const id = jobId.trim();
  if (id && !validId(id)) problems.push('The job ID must be a UUID, for example 22222222-2222-4222-8222-222222222222.');
  if (length(reason) > maxReason) problems.push(`The reason can have at most ${maxReason} characters.`);
  return problems;
}

export const readyToSend = (jobId: string, reason: string, problems: string[]) =>
  validId(jobId.trim()) && length(reason) > 0 && problems.length === 0;
