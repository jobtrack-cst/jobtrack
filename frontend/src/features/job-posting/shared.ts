export const jobMessages = {
  3: 'Job details were not accepted. Check the title, field limits and future deadline.',
  7: 'Only the employer who owns this job can manage it.',
  9: 'This action is no longer allowed. Refresh the job to see its current status.',
};
export const publishMessages = { ...jobMessages, 9: 'Cannot publish. Your company must be verified, the job must be a draft, and its deadline must be in the future. Check Company and Verification, then refresh the job.' };
export const jobStatus = (status: string) => ({ JOB_STATUS_DRAFT: 'Draft', JOB_STATUS_PUBLISHED: 'Published', JOB_STATUS_CLOSED: 'Closed', JOB_STATUS_REMOVED: 'Removed' })[status] ?? 'Unknown';
export const editable = (status: string) => status === 'JOB_STATUS_DRAFT' || status === 'JOB_STATUS_PUBLISHED';
