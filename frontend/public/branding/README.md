# JobTrack branding asset

The shared header uses `frontend/src/components/Brand.tsx`. Its logo mark is
`frontend/public/branding/jobtrack-mark.svg`, served by Vite at
`/branding/jobtrack-mark.svg`. The wordmark stays accessible HTML text.

To use the team's logo, replace `jobtrack-mark.svg` with the local SVG, retaining
that filename. Keep a `viewBox` and use a square or approximately square logo
mark; it is displayed at 38 × 38 CSS pixels with `object-fit: contain`. Include
enough internal contrast for light and dark surroundings. No package change or
backend setup is needed; rebuild the frontend with `npm run build`.

For PNG, add `frontend/public/branding/jobtrack-mark.png` and change `markAsset`
in `Brand.tsx` to `/branding/jobtrack-mark.png`. A transparent PNG at least
76 × 76 pixels works well for high-density screens. Keep the HTML wordmark and
`aria-label="JobTrack home"`; the decorative image deliberately has empty alt
text to avoid repeating the link's accessible name. If loading fails, the
component shows a styled JT monogram and the wordmark.

`weave.svg` is a separate, lightweight decorative repeat. The original valley,
terraces, river, bridge and stepped architectural scene are authored in
`frontend/public/branding/valley.svg` (4.4 kB) and rendered by `Landscape.tsx`.
The artwork is decorative, uses no third-party imagery, and is cropped within
its own figure on small screens. The auth footer and textile strip have separate
flow regions. These are local build-time assets, not upload features.
