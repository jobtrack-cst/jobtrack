import { useId, type ChangeEvent, type FormEvent, type ReactNode } from 'react';
import { Box, Button, InputBase } from '@mui/material';
import { glassFlat, tokens } from '../theme';

// Glass search bar (redesign doc Section 4): segments with a small label above the
// value, and one Search button 60px high. Used by Find jobs and the admin filters.
export function SearchBar({ label, children, submitLabel = 'Search', submitDisabled, onSubmit }: {
  label: string; children: ReactNode; submitLabel?: string; submitDisabled?: boolean; onSubmit: () => void;
}) {
  return <Box component="form" role="search" aria-label={label} noValidate onSubmit={(e: FormEvent) => { e.preventDefault(); onSubmit(); }}
    sx={{ ...glassFlat, borderRadius: '20px', p: 1, display: 'flex', flexWrap: 'wrap', alignItems: 'stretch', gap: 0.5, minWidth: 0 }}>
    {children}
    <Button type="submit" variant="contained" disabled={submitDisabled}
      sx={{ minHeight: 60, borderRadius: '14px', px: 3.5, fontSize: '1rem', flex: { xs: '1 1 100%', md: '0 0 auto' } }}>{submitLabel}</Button>
  </Box>;
}

export function SearchField({ label, value, onChange, type = 'text', placeholder, disabled, grow = 1 }: {
  label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; disabled?: boolean; grow?: number;
}) {
  const id = useId();
  return <Box sx={{ flex: `${grow} 1 200px`, minWidth: 0, px: 2, py: 1.25, borderRadius: '14px', display: 'flex', flexDirection: 'column', gap: 0.25,
    '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
    '&:focus-within': { outline: `3px solid ${tokens.accent}`, outlineOffset: -3, bgcolor: 'rgba(255,255,255,0.04)' } }}>
    <Box component="label" htmlFor={id} sx={{ fontSize: '0.8125rem', fontWeight: 600, color: tokens.textSecondary }}>{label}</Box>
    <InputBase id={id} type={type} value={value} placeholder={placeholder} disabled={disabled}
      onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
      sx={{ fontSize: '1.0625rem', fontWeight: 600, color: tokens.text, '& input': { p: 0, minHeight: 28 },
        '& input::placeholder': { color: tokens.textMuted, opacity: 1 }, '& input::-webkit-calendar-picker-indicator': { filter: 'invert(0.85)' },
        // The bar's own focus ring replaces the input's.
        '& input:focus-visible': { outline: 'none' } }} />
  </Box>;
}
