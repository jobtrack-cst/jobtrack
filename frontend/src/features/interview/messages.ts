import { grpcCode } from '../application-status/shared';

// Fixed messages for listing and scheduling interviews (JT-52 rules).
export const listMessages = {
  [grpcCode.notFound]: 'This application is not available yet. It appears shortly after it is submitted; refresh to try again.',
  [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see its interviews.',
};
export const scheduleMessages = {
  [grpcCode.invalidArgument]: 'The interview was not scheduled. Choose a time in the future within 365 days, with details of up to 1000 characters.',
  [grpcCode.alreadyExists]: 'An interview is already scheduled at this time for this application.',
  [grpcCode.permissionDenied]: 'Only the employer who posted this job can schedule interviews.',
  [grpcCode.notFound]: listMessages[grpcCode.notFound],
};
