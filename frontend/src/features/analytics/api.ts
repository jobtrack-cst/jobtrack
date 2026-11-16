import { api } from '../../api/client';

// int64 counts arrive as strings in protojson and are left out when 0.
type Count = string | number;
export interface StatusCount { status?: string; count?: Count }
export interface Summary {
  startTime?: string;
  endTime?: string;
  jobsPublished?: Count;
  jobsClosed?: Count;
  applicationsSubmitted?: Count;
  applicationsWithdrawn?: Count;
  offersMade?: Count;
  offersAccepted?: Count;
  offersDeclined?: Count;
  statusChanges?: StatusCount[];
}

// start is inclusive and end exclusive, both RFC 3339 in UTC (JT-36).
export const getSummary = (startTime: string, endTime: string) =>
  api<{ summary?: Summary }>(`/v1/analytics/summary?start_time=${encodeURIComponent(startTime)}&end_time=${encodeURIComponent(endTime)}`);
