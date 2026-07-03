import { Link } from "react-router-dom";

export type BandSection = "overview" | "setlists" | "settings";

/**
 * Band section navigation as a tab strip (T04) — replaces the bare Setlists/Settings
 * links. Highlights the active section; Settings is admin-only. Reused across the
 * band overview, setlists, and settings routes so the strip is consistent and the
 * current section is always marked.
 */
export function SectionTabs({
  bandId,
  active,
  showSettings,
}: {
  bandId: string;
  active: BandSection;
  showSettings: boolean;
}) {
  const tab = (to: string, key: BandSection, label: string, testId: string) => (
    <Link
      to={to}
      data-testid={testId}
      className={"section-tab" + (active === key ? " active" : "")}
      aria-current={active === key ? "page" : undefined}
    >
      {label}
    </Link>
  );
  return (
    <nav className="section-tabs" aria-label="Band sections">
      {tab(`/bands/${bandId}`, "overview", "Overview", "nav-overview")}
      {tab(`/bands/${bandId}/setlists`, "setlists", "Setlists", "nav-setlists")}
      {showSettings && tab(`/bands/${bandId}/settings`, "settings", "Settings", "nav-settings")}
    </nav>
  );
}
