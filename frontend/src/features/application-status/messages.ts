import { grpcCode } from './shared';

// Fixed messages for reading an application's status and history (JT-49 access rules).
export const readMessages = {
  [grpcCode.notFound]: 'This application has no status yet. It appears shortly after the application is submitted; refresh to try again.',
  [grpcCode.permissionDenied]: 'Only the applicant and the employer of this application can see its status.',
};
