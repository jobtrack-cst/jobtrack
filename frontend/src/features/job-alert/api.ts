import { api } from '../../api/client';
import { idPath, pageQuery, type Pagination } from '../application/shared';
export interface JobAlert { alertId: string; categoryIds?: string[]; tagIds?: string[]; location?: string }
export interface Category { categoryId: string; name: string }
export interface Tag { tagId: string; name: string }
export interface Location { locationId: string; name: string }
export const listAlerts = (token: string) => api<{ jobAlerts?: JobAlert[]; pagination?: Pagination }>(`/v1/job-alerts?${pageQuery(token)}`);
export const createAlert = (categoryIds: string[], tagIds: string[], location: string) => api<{ jobAlert: JobAlert }>('/v1/job-alerts', { method: 'POST', body: { categoryIds, tagIds, location } });
export const deleteAlert = (id: string) => api<Record<string, never>>(`/v1/job-alerts/${idPath(id)}`, { method: 'DELETE' });
async function reference<T>(path: string, key: string): Promise<T[]> {
  const rows: T[] = [];
  let token = '';
  const seen = new Set<string>();
  do {
    const result = await api<{ [key: string]: unknown; pagination?: Pagination }>(`${path}?${pageQuery(token)}`);
    rows.push(...(result[key] as T[] ?? []));
    token = result.pagination?.nextPageToken ?? '';
    if (token && seen.has(token)) throw new Error('Invalid pagination response. Please refresh.');
    seen.add(token);
  } while (token);
  return rows;
}
export const alertReferences = async () => {
  const [categories, tags, locations] = await Promise.all([reference<Category>('/v1/categories', 'categories'), reference<Tag>('/v1/tags', 'tags'), reference<Location>('/v1/locations', 'locations')]);
  return { categories, tags, locations };
};
