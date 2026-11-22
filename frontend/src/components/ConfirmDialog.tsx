import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack } from '@mui/material';
import { useId, type ReactNode } from 'react';

interface Props {
  open: boolean;
  title: string;
  children?: ReactNode;
  confirmLabel: string;
  destructive?: boolean;
  busy?: boolean;
  // Extra rule, for example a required reason.
  confirmDisabled?: boolean;
  error?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

// Cannot be closed while the action runs, so a late result never lands on a closed dialog.
export function ConfirmDialog({ open, title, children, confirmLabel, destructive, busy, confirmDisabled, error, onConfirm, onCancel }: Props) {
  const titleId = useId();
  return <Dialog open={open} aria-labelledby={titleId} onClose={() => { if (!busy) onCancel(); }}>
    <DialogTitle id={titleId}>{title}</DialogTitle>
    <DialogContent><Stack spacing={2} sx={{ pt: 0.5 }}>
      {children}
      {error && <Alert severity="error">{error}</Alert>}
    </Stack></DialogContent>
    <DialogActions>
      <Button disabled={busy} onClick={onCancel}>Cancel</Button>
      <Button variant="contained" color={destructive ? 'error' : 'primary'} disabled={busy || confirmDisabled} onClick={onConfirm}>
        {busy ? 'Please wait…' : confirmLabel}
      </Button>
    </DialogActions>
  </Dialog>;
}
