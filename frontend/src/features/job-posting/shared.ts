import { useLoad } from '../application-status/useLoad';
import { alertReferences, type Category, type Tag } from '../job-alert/api';

export const jobMessages = {
  3: 'Job details were not accepted. Check the title, field limits and future deadline.',
  7: 'Only the employer who owns this job can manage it.',
  9: 'This action is no longer allowed. Refresh the job to see its current status.',
};
export const publishMessages = { ...jobMessages, 9: 'Cannot publish. Your company must be verified, the job must be a draft, and its deadline must be in the future. Check your Company page, then refresh the job.' };
export const deleteMessages = {
  ...jobMessages,
  5: 'This draft no longer exists. Open Jobs to see your drafts.',
  9: 'Only drafts can be deleted. Refresh the job to see its current status.',
};
export const editable =(status: string) => status === 'JOB_STATUS_DRAFT' || status === 'JOB_STATUS_PUBLISHED';

// Categories, tags and locations (category-tag, every page) for the job form and details.
export const useJobReferences = () => useLoad(alertReferences, [], jobMessages);

// A category or tag the reference lists do not know (for example one removed later) is
// shown by these labels and never by its ID; the form keeps its value when saving.
export const unknownCategory = 'Unknown category';
export const unknownTag = 'Unknown tag';
export const categoryName = (categories: Category[], id?: string) =>
  !id ? 'No category' : categories.find(c => c.categoryId === id)?.name ?? unknownCategory;
export const tagName = (tags: Tag[], id: string) => tags.find(t => t.tagId === id)?.name ?? unknownTag;
