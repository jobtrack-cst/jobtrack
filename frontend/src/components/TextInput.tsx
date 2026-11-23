import { useId, type ChangeEvent, type ReactNode } from 'react';
import { Box, FormHelperText, FormLabel, OutlinedInput, Select } from '@mui/material';
import { tokens } from '../theme';

// Field style shared by the login page, form panels and dialogs (redesign doc Section 2):
// label above (14px, 600, white), 48px input, radius 12, 3px accent focus outline.
export const inputSx = {
  minHeight: 48, borderRadius: '12px', bgcolor: tokens.surface, color: tokens.text, fontSize: '1rem',
  '& .MuiOutlinedInput-notchedOutline': { borderColor: tokens.border, borderWidth: '1px !important' },
  '&.Mui-focused': { outline: `3px solid ${tokens.accent}`, outlineOffset: 2 },
  '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: tokens.accent },
  '&.Mui-error .MuiOutlinedInput-notchedOutline': { borderColor: tokens.danger },
  '& .MuiOutlinedInput-input': { py: '12px', px: '14px' },
  '& .MuiOutlinedInput-inputMultiline': { p: 0 },
  '&.MuiInputBase-multiline': { py: '12px', px: '14px' },
  '& input::placeholder, & textarea::placeholder': { color: tokens.textMuted, opacity: 1 },
  // Light picker icons on the dark input.
  '& input::-webkit-calendar-picker-indicator': { filter: 'invert(0.85)' },
} as const;

interface Props {
  label: string;
  name?: string;
  type?: string;
  value?: string;
  defaultValue?: string;
  onChange?: (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  required?: boolean;
  disabled?: boolean;
  autoComplete?: string;
  placeholder?: string;
  maxLength?: number;
  inputMode?: 'numeric' | 'text' | 'email';
  multiline?: boolean;
  minRows?: number;
  error?: boolean;
  helperText?: ReactNode;
  // Options for a select (MenuItem elements).
  select?: boolean;
  children?: ReactNode;
}

export function TextInput({ label, name, type = 'text', value, defaultValue, onChange, required, disabled, autoComplete, placeholder, maxLength, inputMode,
  multiline, minRows, error, helperText, select, children }: Props) {
  const id = useId();
  const labelId = `${id}-label`;
  const helperId = helperText ? `${id}-helper` : undefined;
  return <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75, minWidth: 0 }}>
    <FormLabel id={labelId} htmlFor={select ? undefined : id} required={required} error={false}
      sx={{ color: tokens.text, fontSize: '0.875rem', fontWeight: 600, '&.Mui-focused': { color: tokens.text }, '& .MuiFormLabel-asterisk': { color: tokens.textSecondary } }}>
      {label}
    </FormLabel>
    {select
      ? <Select id={id} labelId={labelId} name={name} value={value} defaultValue={defaultValue} disabled={disabled} error={error}
        onChange={onChange as never} input={<OutlinedInput sx={inputSx} />} aria-describedby={helperId}>{children}</Select>
      : <OutlinedInput id={id} name={name} type={type} value={value} defaultValue={defaultValue} onChange={onChange} required={required}
        disabled={disabled} autoComplete={autoComplete} placeholder={placeholder} multiline={multiline} minRows={minRows} error={error}
        inputProps={{ maxLength, inputMode, 'aria-describedby': helperId }} sx={inputSx} />}
    {helperText && <FormHelperText id={helperId} error={error} sx={{ m: 0, color: error ? tokens.danger : tokens.textMuted }}>{helperText}</FormHelperText>}
  </Box>;
}
