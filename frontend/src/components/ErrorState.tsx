import { Alert, Button } from '@mui/material';

// `message` must already be a fixed message (gatewayMessage), never server text.
export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <Alert severity="error" action={onRetry && <Button color="inherit" onClick={onRetry}>Try again</Button>}>{message}</Alert>;
}
