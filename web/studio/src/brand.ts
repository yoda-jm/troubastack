/**
 * BRAND03 / BRAND11: the one canonical link back to the public project page.
 *
 * It is shown in two places — the account menu (logged-in, BRAND03) and the auth screens
 * (logged-out, BRAND11) — so the URL lives here once. A signed-out visitor handed a self-hosted
 * server's URL has no account menu; the auth-screen link is their only route to "what is this?".
 */
export const PROJECT_PAGE_URL = "https://yoda-jm.github.io/troubastack/";
