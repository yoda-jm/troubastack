/**
 * BRAND08 (login half): the full TroubaStudio wordmark, shown at the top of the auth screens —
 * the first thing a user sees, and (TroubaStack being self-hosted) where "which server am I signing
 * into?" is answered.
 *
 * The wordmark ships as two ground-specific assets (`-wordmark` / `-wordmark-dark`), so the swap is by
 * COLOUR SCHEME, not by tinting one SVG. The Studio has no manual theme toggle — it follows the OS via
 * `prefers-color-scheme` — so the two <img>s are toggled by that media query in styles.css.
 *
 * Both SVGs are now PURE PATHS (BRAND06 part 2 outlined the type), so they carry NO text of their own:
 * the accessible name is supplied here, once, by the wrapper's role="img" + aria-label; the images are
 * decorative (alt="" aria-hidden). Served from docs/brand/dist by the brandAssets Vite plugin — no copy
 * committed under web/studio.
 */
export function AuthWordmark() {
  return (
    <div className="auth-wordmark" role="img" aria-label="TroubaStudio">
      <img className="wm-light" src="/troubastudio-wordmark.svg" alt="" aria-hidden="true" />
      <img className="wm-dark" src="/troubastudio-wordmark-dark.svg" alt="" aria-hidden="true" />
    </div>
  );
}
