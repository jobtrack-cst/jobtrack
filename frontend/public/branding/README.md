# JobTrack branding and static assets

All files are local and served by Vite from `frontend/public/`. Nothing is
requested from a third party at runtime (redesign doc Section 2).

| File | Use |
|---|---|
| `branding/jobtrack-mark.svg` | Logo mark (rounded `accent` square, `#0E1A3B` "J", white dot) and the favicon (`index.html`). `Brand.tsx` shows it next to the HTML wordmark "JobTrack". |
| `branding/background-night.svg` | Fixed full-page background (indigo and teal glow over `canvas` `#151B2C`), drawn by the theme on `body::before`. Decorative only. The glows are radial gradients; do not use an SVG blur filter here: without a GPU (Cypress in WSL) it is the most expensive repaint on the page and the likely cause of slow screenshots. |
| `fonts/plus-jakarta-sans-latin-wght-normal.woff2` | Plus Jakarta Sans, variable weight 200 to 800, latin subset, loaded with `@font-face` in `src/theme.ts` (`font-display: swap`). |
| `fonts/OFL.txt` | SIL Open Font License 1.1 for Plus Jakarta Sans. Keep it next to the font. |

The font file is the latin variable build published by Fontsource
(`@fontsource-variable/plus-jakarta-sans` 5.2.5), copied in as a static file;
it is not an npm dependency.

To replace the logo, keep the filename and a square `viewBox`. The mark is
decorative next to the wordmark, so `Brand.tsx` gives the image empty alt text
and the link carries `aria-label="JobTrack home"`.
