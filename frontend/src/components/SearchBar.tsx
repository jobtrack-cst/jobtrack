import { useId, type ChangeEvent, type FormEvent, type ReactNode } from 'react';
import { Autocomplete, Box, Button, Checkbox, InputBase, NativeSelect } from '@mui/material';
import { glassFlat, tokens } from '../theme';
import { Icon } from './Icon';

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

// Segments stack on phones and share a row from 600px up.
const segmentSx = (grow: number) => ({ flex: { xs: '1 1 100%', sm: `${grow} 1 200px` }, minWidth: 0, px: 2, py: 1.25, borderRadius: '14px', display: 'flex', flexDirection: 'column', gap: 0.25,
  '&:hover': { bgcolor: 'rgba(255,255,255,0.04)' },
  '&:focus-within': { outline: `3px solid ${tokens.accent}`, outlineOffset: -3, bgcolor: 'rgba(255,255,255,0.04)' } }) as const;
const labelSx = { fontSize: '0.8125rem', fontWeight: 600, color: tokens.textSecondary } as const;
const valueSx = { fontSize: '1.0625rem', fontWeight: 600, color: tokens.text, '& input, & select': { p: 0, minHeight: 28 },
  '& input::placeholder': { color: tokens.textMuted, opacity: 1 }, '& input::-webkit-calendar-picker-indicator': { filter: 'invert(0.85)' },
  // Native option lists use the dark surface too.
  '& option': { bgcolor: tokens.surface, color: tokens.text },
  // The bar's own focus ring replaces the input's.
  '& input:focus-visible, & select:focus-visible': { outline: 'none' } } as const;

// `suggestions` become a native datalist, so any text can still be typed.
export function SearchField({ label, value, onChange, type = 'text', placeholder, disabled, grow = 1, suggestions }: {
  label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; disabled?: boolean; grow?: number;
  suggestions?: string[];
}) {
  const id = useId();
  const listId = suggestions?.length ? `${id}-list` : undefined;
  return <Box sx={segmentSx(grow)}>
    <Box component="label" htmlFor={id} sx={labelSx}>{label}</Box>
    <InputBase id={id} type={type} value={value} placeholder={placeholder} disabled={disabled} inputProps={{ list: listId }}
      onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)} sx={valueSx} />
    {listId && <datalist id={listId}>{suggestions!.map(s => <option key={s} value={s} />)}</datalist>}
  </Box>;
}

// A native select in the same segment style; the first option is the "any" choice.
export function SearchSelect({ label, value, onChange, options, anyLabel = 'Any', disabled, grow = 1 }: {
  label: string; value: string; onChange: (value: string) => void; options: [value: string, label: string][]; anyLabel?: string;
  disabled?: boolean; grow?: number;
}) {
  const id = useId();
  return <Box sx={segmentSx(grow)}>
    <Box component="label" htmlFor={id} sx={labelSx}>{label}</Box>
    <NativeSelect value={value} disabled={disabled} inputProps={{ id }} onChange={(e: ChangeEvent<HTMLSelectElement>) => onChange(e.target.value)}
      input={<InputBase sx={[valueSx, { '& select': { cursor: 'pointer', pr: '28px !important' }, '& .MuiNativeSelect-icon': { color: tokens.textSecondary } }]} />}>
      <option value="">{anyLabel}</option>
      {options.map(([v, text]) => <option key={v} value={v}>{text}</option>)}
    </NativeSelect>
  </Box>;
}

// Multiple choice with checkboxes; typing filters the options. The field itself shows
// "Any" or "N selected"; the page shows the chosen items (for example as chips).
export function SearchMultiSelect<T>({ label, options, value, onChange, optionLabel, optionKey, disabled, grow = 1 }: {
  label: string; options: T[]; value: T[]; onChange: (value: T[]) => void; optionLabel: (option: T) => string; optionKey: (option: T) => string;
  disabled?: boolean; grow?: number;
}) {
  const id = useId();
  const summary = value.length ? `${value.length} selected` : 'Any';
  return <Box sx={segmentSx(grow)}>
    <Box component="label" htmlFor={id} sx={labelSx}>{label}</Box>
    <Autocomplete multiple disableCloseOnSelect id={id} options={options} value={value} disabled={disabled}
      getOptionLabel={optionLabel} isOptionEqualToValue={(a, b) => optionKey(a) === optionKey(b)}
      onChange={(_, next) => onChange(next)}
      // The chosen items are shown by the page, not inside the field.
      renderValue={() => null}
      renderOption={(props, option, { selected }) => {
        const { key, ...rest } = props as typeof props & { key: string };
        return <li key={key} {...rest}><Checkbox checked={selected} size="small" sx={{ mr: 1, p: 0.5 }} tabIndex={-1} disableRipple />{optionLabel(option)}</li>;
      }}
      renderInput={params => <InputBase ref={params.InputProps.ref} inputProps={params.inputProps} disabled={params.disabled} fullWidth
        // The summary is the placeholder, so typing to filter replaces it.
        placeholder={summary}
        endAdornment={<Icon name="chevronDown" size={18} sx={{ color: tokens.textSecondary, pointerEvents: 'none' }} />}
        sx={[valueSx, { '& input::placeholder': { color: value.length ? tokens.text : tokens.textMuted, opacity: 1 } }]} />} />
  </Box>;
}
