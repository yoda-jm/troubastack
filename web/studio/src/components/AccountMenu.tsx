/**
 * T58 — the topbar account menu: ONE top-right trigger (avatar + display name,
 * avatar-only at phone width) opening a dropdown that consolidates what used to be
 * three separate affordances (the profile link, the standalone GetAppChip, and the
 * VersionChip):
 *   1. My account → /me
 *   2. Get the app → opens the EXISTING QR/download popover (reused, not inlined —
 *      the QR needs room); the item is hidden when /api/apps is empty.
 *   3. Footer: the Studio/server build line + the version-mismatch warning.
 *   4. Log out.
 * When the version-mismatch warning is active a glanceable dot rides the trigger, so
 * the urgent signal is not buried inside a closed menu (the /api/version fetch is
 * therefore eager, not click-lazy as the old chip was).
 *
 * Invites/Bands deliberately stay in the left-side nav (navigation with a badge, not
 * account state). Suppressed with the whole topbar in fullscreen/embedded (Shell).
 */
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import QRCode from "qrcode";
import { api, type AppBinary } from "../api";
import { Avatar } from "./Avatar";

type ServerVersion = { version: string; builtAt: string; spaEmbedded: boolean };
type MenuUser = { displayName: string; avatarKind?: import("../api").AvatarKind };

export function AccountMenu({ user, onLogout }: { user: MenuUser; onLogout: () => void }) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [appOpen, setAppOpen] = useState(false);
  const [server, setServer] = useState<ServerVersion | null>(null);
  const [apps, setApps] = useState<AppBinary[]>([]);
  const wrapRef = useRef<HTMLDivElement>(null);
  const qrRef = useRef<HTMLDivElement>(null);

  // Eager version fetch: the mismatch dot must be glanceable BEFORE the menu opens
  // (the old VersionChip fetched lazily on click — no good for a trigger badge).
  useEffect(() => {
    let cancelled = false;
    fetch("/api/version")
      .then((r) => r.json())
      .then((v: ServerVersion) => {
        if (!cancelled) setServer(v);
      })
      .catch(() => {
        /* version unavailable — no dot, footer shows "…" */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Apps manifest: the "Get the app" item is hidden entirely when empty (dev / no-embed).
  useEffect(() => {
    let cancelled = false;
    api
      .listApps()
      .then((a) => {
        if (!cancelled) setApps(a);
      })
      .catch(() => {
        /* no apps endpoint / none embedded — the item stays hidden */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Close on click-outside or Escape (covers both the menu and the app panel).
  useEffect(() => {
    if (!menuOpen && !appOpen) return;
    function onDocPointer(e: PointerEvent) {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
        setAppOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setMenuOpen(false);
        setAppOpen(false);
      }
    }
    document.addEventListener("pointerdown", onDocPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDocPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [menuOpen, appOpen]);

  const android = apps.find((a) => a.platform === "android");
  const ios = apps.find((a) => a.platform === "ios");
  const primary = android ?? ios; // the QR points at the primary download
  const url = primary ? window.location.origin + primary.path : "";

  // Render the QR only while the app panel is open (reuses the OPS02 popover verbatim).
  useEffect(() => {
    if (!appOpen || !url) return;
    let cancelled = false;
    QRCode.toString(url, { type: "svg", margin: 1, width: 132 })
      .then((svg) => {
        if (!cancelled && qrRef.current) qrRef.current.innerHTML = svg;
      })
      .catch(() => {
        if (!cancelled && qrRef.current) qrRef.current.textContent = url;
      });
    return () => {
      cancelled = true;
    };
  }, [appOpen, url]);

  const mismatch = server != null && server.version !== __APP_VERSION__;
  const meta = (a: AppBinary) => `${a.version} · ${(a.size / 1e6).toFixed(1)} MB`;

  return (
    <div className="account-menu-wrap" ref={wrapRef}>
      <button
        type="button"
        className="account-trigger"
        data-testid="account-trigger"
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        onClick={() => {
          setAppOpen(false);
          setMenuOpen((o) => !o);
        }}
      >
        <Avatar user={user} size={26} />
        <span className="account-name" data-testid="current-user">
          {user.displayName}
        </span>
        {mismatch && (
          <span
            className="account-dot"
            data-testid="account-warning-dot"
            title="Version mismatch — open the menu"
            aria-label="Attention needed"
          />
        )}
      </button>

      {menuOpen && (
        <div className="account-menu" data-testid="account-menu" role="menu">
          <Link
            to="/me"
            role="menuitem"
            className="account-item"
            data-testid="menu-account"
            onClick={() => setMenuOpen(false)}
          >
            <span aria-hidden="true">👤</span> My account
          </Link>

          {apps.length > 0 && (
            <button
              type="button"
              role="menuitem"
              className="account-item"
              data-testid="get-app-btn"
              aria-expanded={appOpen}
              onClick={() => {
                setMenuOpen(false);
                setAppOpen(true);
              }}
            >
              <span aria-hidden="true">📱</span> Get the app
            </button>
          )}

          <button
            type="button"
            role="menuitem"
            className="account-item"
            data-testid="logout"
            onClick={() => {
              setMenuOpen(false);
              onLogout();
            }}
          >
            <span aria-hidden="true">↪</span> Log out
          </button>

          {/* Footer: build identity + the mismatch warning (the retired VersionChip's
              body, now always shown with the menu — no separate click). */}
          <div className="account-version" data-testid="version-popover">
            <div className="mono">Studio&nbsp;&nbsp;{__APP_VERSION__}</div>
            {server ? (
              <>
                <div className="mono" data-testid="version-server">
                  Server&nbsp;&nbsp;{server.version} · {server.builtAt}
                  {!server.spaEmbedded && " · no SPA embedded"}
                </div>
                {mismatch && (
                  <div className="version-mismatch" data-testid="version-mismatch" role="alert">
                    ⚠ Studio and server versions differ — reload (Ctrl+Shift+R) or rebuild the server.
                  </div>
                )}
              </>
            ) : (
              <div className="mono muted">…</div>
            )}
          </div>
        </div>
      )}

      {/* The reused QR/download panel (OPS02) — opened from the "Get the app" item. */}
      {appOpen && apps.length > 0 && (
        <div className="getapp-popover account-app-popover" data-testid="get-app-popover">
          <div className="getapp-title">Get TroubaStage</div>
          <p className="getapp-sub">
            Perform offline on your phone or tablet. Scan the code with your phone camera, or tap
            to download.
          </p>
          {url && <div className="qr" data-testid="get-app-qr" ref={qrRef} />}
          <ul className="getapp-platforms">
            {android && (
              <li className="getapp-platform" data-testid="get-app-android">
                <a
                  className="primary button"
                  data-testid="get-app-download"
                  href={android.path}
                  download={android.filename}
                >
                  Download for Android
                </a>
                <span className="getapp-meta" data-testid="get-app-version">
                  {meta(android)} · Android
                </span>
              </li>
            )}
            <li className="getapp-platform" data-testid="get-app-ios">
              {ios ? (
                <>
                  <a
                    className="primary button"
                    data-testid="get-app-ios-download"
                    href={ios.path}
                    download={ios.filename}
                  >
                    Download for iOS
                  </a>
                  <span className="getapp-meta">{meta(ios)} · iOS</span>
                </>
              ) : (
                <>
                  <span className="button is-disabled" data-testid="get-app-ios-soon" aria-disabled="true">
                    iOS <span className="coming-soon">Coming soon</span>
                  </span>
                  <span className="getapp-meta">Available in a future release</span>
                </>
              )}
            </li>
          </ul>
        </div>
      )}
    </div>
  );
}
