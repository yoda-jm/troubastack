/**
 * BRAND11: the outgoing "About TroubaStudio" link on the logged-out auth screens (Login/Register).
 *
 * A signed-out visitor never sees the account menu (Shell's user area), so BRAND03's link there is
 * unreachable for them — yet they are the one most likely to ask "what is this software?" after being
 * handed a self-hosted server's URL. Same label and new-tab treatment as the account-menu entry; the
 * URL is the single `PROJECT_PAGE_URL` constant so it lives in one place per product.
 */
import { PROJECT_PAGE_URL } from "../brand";

export function AboutLink() {
  return (
    <p className="auth-about">
      <a
        href={PROJECT_PAGE_URL}
        target="_blank"
        rel="noopener noreferrer"
        data-testid="auth-about"
      >
        <span aria-hidden="true">ℹ️</span> About TroubaStudio ↗
      </a>
    </p>
  );
}
