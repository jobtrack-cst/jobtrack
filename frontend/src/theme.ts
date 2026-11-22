import { alpha, createTheme } from '@mui/material/styles';

// Night visual system in the Daylight glass layout (docs/architecture/frontend-redesign.md Section 2).
// Radius: 20 for cards and panels, 12 to 14 for controls and buttons, fully round for small pills.
export const tokens = {
  canvas: '#151B2C',
  glass: 'rgba(20,26,45,0.55)',
  glassBorder: 'rgba(255,255,255,0.12)',
  surface: '#1E2640',
  surfaceRaised: '#26304F',
  border: '#3A4566',
  text: '#FFFFFF',
  textSecondary: '#B8C2DC',
  textMuted: '#8E99B8',
  accent: '#8EA2FF',
  accentHover: '#A9B8FF',
  onAccent: '#0E1A3B',
  success: '#5FD3A5',
  danger: '#FF8A80',
  urgentBg: '#FFE2D2',
  urgentText: '#8A2A0B',
  chipBg: '#2B3560',
  chipText: '#D6DDFF',
} as const;

export const fontFamily = '"Plus Jakarta Sans", "Segoe UI", system-ui, -apple-system, sans-serif';
const motion = '180ms ease';
const focusRing = { outline: `3px solid ${tokens.accent}`, outlineOffset: 2 };
const menuShadow = '0 12px 40px rgba(5,8,20,0.55)';

// Blurred glass only for the few floating surfaces: top bar, dialogs, drawer and the
// auth card. Blur is costly to paint without a GPU, so other glass panels (search
// bars) use the flat version, which looks the same over the smooth background.
export const glass = {
  backgroundColor: tokens.glass,
  backdropFilter: 'blur(18px)',
  WebkitBackdropFilter: 'blur(18px)',
  border: `1px solid ${tokens.glassBorder}`,
} as const;
export const glassFlat = {
  backgroundColor: 'rgba(20,26,45,0.72)',
  border: `1px solid ${tokens.glassBorder}`,
} as const;

export const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: tokens.accent, light: tokens.accentHover, dark: tokens.accent, contrastText: tokens.onAccent },
    secondary: { main: tokens.chipText, light: tokens.chipBg, dark: tokens.chipText, contrastText: tokens.onAccent },
    success: { main: tokens.success, contrastText: tokens.onAccent },
    error: { main: tokens.danger, contrastText: tokens.onAccent },
    warning: { main: tokens.urgentBg, contrastText: tokens.urgentText },
    info: { main: tokens.accent, contrastText: tokens.onAccent },
    background: { default: tokens.canvas, paper: tokens.surface },
    text: { primary: tokens.text, secondary: tokens.textSecondary, disabled: tokens.textMuted },
    divider: tokens.border,
    action: { hover: alpha('#FFFFFF', 0.06), selected: tokens.surfaceRaised },
  },
  typography: {
    fontFamily,
    // Page title 40/48 800, section 22 800, card 18 700, body 16, secondary 14, smallest 13.
    h1: { fontSize: '2.5rem', lineHeight: 1.2, fontWeight: 800, letterSpacing: '-0.04em' },
    h2: { fontSize: '1.375rem', lineHeight: 1.35, fontWeight: 800, letterSpacing: '-0.02em' },
    h3: { fontSize: '1.125rem', lineHeight: 1.4, fontWeight: 700 },
    // Pages not rebuilt yet use h4 as their title and h6 for sections.
    h4: { fontSize: '2.5rem', lineHeight: 1.2, fontWeight: 800, letterSpacing: '-0.04em' },
    h5: { fontSize: '1.375rem', lineHeight: 1.35, fontWeight: 800, letterSpacing: '-0.02em' },
    h6: { fontSize: '1.125rem', lineHeight: 1.4, fontWeight: 700 },
    body1: { fontSize: '1rem', lineHeight: 1.6 },
    body2: { fontSize: '0.875rem', lineHeight: 1.55 },
    subtitle1: { fontSize: '1rem', fontWeight: 600 },
    subtitle2: { fontSize: '0.875rem', fontWeight: 600 },
    caption: { fontSize: '0.8125rem', lineHeight: 1.5 },
    button: { fontSize: '0.9375rem', fontWeight: 700, textTransform: 'none', letterSpacing: 0 },
    // Sentence case, no letter spacing (no eyebrows).
    overline: { fontSize: '0.8125rem', fontWeight: 600, textTransform: 'none', letterSpacing: 0, lineHeight: 1.5 },
  },
  shape: { borderRadius: 12 },
  components: {
    MuiCssBaseline: { styleOverrides: {
      '@font-face': {
        fontFamily: 'Plus Jakarta Sans',
        fontStyle: 'normal',
        fontDisplay: 'swap',
        fontWeight: '200 800',
        src: 'url(/fonts/plus-jakarta-sans-latin-wght-normal.woff2) format("woff2")',
        unicodeRange: 'U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD',
      },
      '*': { boxSizing: 'border-box' },
      html: { colorScheme: 'dark' },
      body: { minWidth: 0, backgroundColor: tokens.canvas, color: tokens.text },
      // A fixed layer instead of background-attachment: fixed, which mobile Safari ignores.
      'body::before': {
        content: '""', position: 'fixed', inset: 0, zIndex: -1, pointerEvents: 'none',
        background: `${tokens.canvas} url(/branding/background-night.svg) center / cover no-repeat`,
      },
      '::selection': { background: alpha(tokens.accent, 0.35) },
      ':focus-visible': focusRing,
      'a, button, input, textarea': { WebkitTapHighlightColor: 'transparent' },
      '@media (prefers-reduced-motion: reduce)': {
        '*, *::before, *::after': { animation: 'none !important', transition: 'none !important', scrollBehavior: 'auto !important' },
      },
    } },
    MuiStack: { defaultProps: { useFlexGap: true } },
    MuiButtonBase: { defaultProps: { disableRipple: true } },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: { borderRadius: 12, minHeight: 44, padding: '10px 18px', lineHeight: 1.4, transition: `background-color ${motion}, border-color ${motion}, color ${motion}`,
          '&.Mui-focusVisible': focusRing },
        containedPrimary: { backgroundColor: tokens.accent, color: tokens.onAccent, '&:hover': { backgroundColor: tokens.accentHover },
          '&.Mui-disabled': { backgroundColor: tokens.surfaceRaised, color: tokens.textMuted } },
        outlined: { borderColor: tokens.border, '&:hover': { borderColor: tokens.accent, backgroundColor: alpha(tokens.accent, 0.08) } },
        outlinedPrimary: { color: tokens.accent },
        text: { '&:hover': { backgroundColor: alpha(tokens.accent, 0.1) } },
        sizeSmall: { minHeight: 44, padding: '8px 14px' },
      },
    },
    MuiIconButton: { styleOverrides: { root: { borderRadius: 12, minWidth: 44, minHeight: 44, '&.Mui-focusVisible': focusRing } } },
    MuiPaper: { defaultProps: { elevation: 0 }, styleOverrides: {
      // Dark mode adds a white overlay to elevated paper; Night uses flat surfaces.
      root: { backgroundImage: 'none' },
      outlined: { borderColor: tokens.border },
    } },
    MuiCard: {
      defaultProps: { variant: 'outlined' },
      styleOverrides: { root: { borderRadius: 20, backgroundColor: tokens.surface, borderColor: tokens.border, boxShadow: 'none', overflowWrap: 'anywhere' } },
    },
    MuiCardContent: { styleOverrides: { root: { padding: 24, '&:last-child': { paddingBottom: 24 } } } },
    MuiCardActionArea: { styleOverrides: { root: { '&.Mui-focusVisible': { outline: `3px solid ${tokens.accent}`, outlineOffset: -3 },
      '&:hover': { backgroundColor: tokens.surfaceRaised } }, focusHighlight: { display: 'none' } } },
    MuiCardActions: { styleOverrides: { root: { padding: '0 20px 20px', gap: 8, flexWrap: 'wrap' } } },
    MuiTextField: { defaultProps: { variant: 'outlined' } },
    MuiOutlinedInput: { styleOverrides: {
      root: { backgroundColor: tokens.surface, borderRadius: 12,
        '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: tokens.textMuted },
        '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: tokens.accent, borderWidth: 2 } },
      notchedOutline: { borderColor: tokens.border },
      input: { minHeight: 24 },
    } },
    MuiInputLabel: { styleOverrides: { root: { color: tokens.textSecondary, '&.Mui-focused': { color: tokens.accent } } } },
    MuiFormHelperText: { styleOverrides: { root: { marginLeft: 0, fontSize: '0.8125rem', color: tokens.textMuted } } },
    MuiAutocomplete: { styleOverrides: {
      root: {
        // MUI reserves 26px per icon; our 44px targets need the same space in the input.
        '&.MuiAutocomplete-hasPopupIcon .MuiOutlinedInput-root, &.MuiAutocomplete-hasClearIcon .MuiOutlinedInput-root': { paddingRight: 44 + 4 + 9 },
        '&.MuiAutocomplete-hasPopupIcon.MuiAutocomplete-hasClearIcon .MuiOutlinedInput-root': { paddingRight: 88 + 4 + 9 },
      },
      paper: { backgroundColor: tokens.surfaceRaised, border: `1px solid ${tokens.border}`, boxShadow: menuShadow },
    } },
    MuiMenu: { styleOverrides: { paper: { marginTop: 6, backgroundColor: tokens.surfaceRaised, border: `1px solid ${tokens.border}`, borderRadius: 12, boxShadow: menuShadow } } },
    MuiPopover: { styleOverrides: { paper: { backgroundColor: tokens.surfaceRaised } } },
    MuiMenuItem: { styleOverrides: { root: { borderRadius: 8, minHeight: 44, margin: '2px 6px', whiteSpace: 'normal', overflowWrap: 'anywhere',
      // MUI drops the minimum height to auto from 600px up; touch targets stay 44px.
      '@media (min-width: 600px)': { minHeight: 44 },
      '&:hover': { backgroundColor: alpha('#FFFFFF', 0.06) },
      '&.Mui-selected': { backgroundColor: alpha(tokens.accent, 0.18), fontWeight: 700 },
      '&.Mui-focusVisible': { outline: `2px solid ${tokens.accent}`, outlineOffset: -2 } } } },
    MuiChip: { defaultProps: { size: 'small' }, styleOverrides: {
      root: { borderRadius: 999, fontWeight: 600, fontSize: '0.8125rem', height: 'auto', minHeight: 28, maxWidth: '100%' },
      label: { padding: '4px 12px', whiteSpace: 'normal', overflowWrap: 'anywhere' },
      colorDefault: { backgroundColor: tokens.chipBg, color: tokens.chipText },
      clickable: { '&.Mui-focusVisible': focusRing },
    } },
    MuiAlert: { styleOverrides: {
      root: { borderRadius: 14, alignItems: 'center', border: `1px solid ${tokens.border}`, backgroundColor: tokens.surface, color: tokens.text },
      message: { overflowWrap: 'anywhere' },
      standardSuccess: { borderColor: alpha(tokens.success, 0.5), '& .MuiAlert-icon': { color: tokens.success } },
      standardError: { borderColor: alpha(tokens.danger, 0.6), '& .MuiAlert-icon': { color: tokens.danger } },
      standardWarning: { borderColor: alpha(tokens.urgentBg, 0.5), '& .MuiAlert-icon': { color: tokens.urgentBg } },
      standardInfo: { borderColor: alpha(tokens.accent, 0.5), '& .MuiAlert-icon': { color: tokens.accent } },
    } },
    MuiTable: { styleOverrides: { root: { tableLayout: 'fixed', backgroundColor: tokens.surface, borderCollapse: 'separate', borderSpacing: 0,
      border: `1px solid ${tokens.border}`, borderRadius: 20, overflow: 'hidden' } } },
    MuiTableCell: { styleOverrides: {
      root: { padding: '12px 16px', borderColor: tokens.border, overflowWrap: 'anywhere', fontSize: '0.875rem' },
      head: { color: tokens.textSecondary, fontWeight: 700 },
      alignRight: { fontVariantNumeric: 'tabular-nums' },
    } },
    MuiTableRow: { styleOverrides: { root: { '&:last-child td, &:last-child th': { borderBottom: 0 }, '&:hover': { backgroundColor: tokens.surfaceRaised } } } },
    MuiTabs: { styleOverrides: { indicator: { height: 3, borderRadius: 3, backgroundColor: tokens.accent } } },
    MuiTab: { styleOverrides: { root: { textTransform: 'none', fontWeight: 700, fontSize: '0.9375rem', minHeight: 48, color: tokens.textSecondary,
      '&.Mui-selected': { color: tokens.text }, '&.Mui-focusVisible': { outline: `3px solid ${tokens.accent}`, outlineOffset: -3 } } } },
    MuiDialog: { defaultProps: { fullWidth: true, maxWidth: 'sm', transitionDuration: 180 }, styleOverrides: {
      paper: { ...glass, backgroundColor: 'rgba(30,38,64,0.92)', borderRadius: 20, boxShadow: '0 24px 80px rgba(5,8,20,0.6)' },
    } },
    MuiDialogTitle: { styleOverrides: { root: { fontWeight: 800, fontSize: '1.375rem', letterSpacing: '-0.02em', paddingTop: 24 } } },
    MuiDialogActions: { styleOverrides: { root: { padding: '16px 24px 24px', flexWrap: 'wrap', gap: 8 } } },
    MuiDrawer: { styleOverrides: { paper: { ...glass, backgroundColor: 'rgba(20,26,45,0.92)', borderLeft: 0, borderTop: 0, borderBottom: 0 } } },
    MuiLink: { defaultProps: { underline: 'hover' }, styleOverrides: { root: { color: tokens.accent, fontWeight: 600, textUnderlineOffset: 4,
      '&:hover': { color: tokens.accentHover } } } },
    MuiLinearProgress: { styleOverrides: { root: { borderRadius: 4, backgroundColor: tokens.surfaceRaised } } },
    MuiSkeleton: { defaultProps: { animation: 'wave' }, styleOverrides: { root: { backgroundColor: tokens.surfaceRaised } } },
    MuiDivider: { styleOverrides: { root: { borderColor: tokens.border } } },
    MuiTooltip: { styleOverrides: { tooltip: { backgroundColor: tokens.surfaceRaised, border: `1px solid ${tokens.border}`, fontSize: '0.8125rem' } } },
  },
});
