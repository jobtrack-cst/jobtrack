import { api } from '../../api/client';
import { pageQuery, type Pagination } from '../application/shared';
import type { Job } from '../job-posting/api';
export interface SearchFilters { query: string; categoryId: string; tagIds: string[]; location: string }
export const emptyFilters: SearchFilters = { query: '', categoryId: '', tagIds: [], location: '' };
export function searchJobs(token: string, filters: SearchFilters) {
  const query = new URLSearchParams({ query: filters.query, category_id: filters.categoryId, location: filters.location });
  filters.tagIds.forEach(id => query.append('tag_ids', id));
  return api<{ jobs?: Omit<Job, 'description' | 'status'>[]; pagination?: Pagination }>(`/v1/search/jobs?${pageQuery(token)}&${query}`);
}
