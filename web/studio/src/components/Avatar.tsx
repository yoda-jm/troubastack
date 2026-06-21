/**
 * Avatar — an inline SVG silhouette keyed by avatarKind. No network, no images:
 * the head+shoulders shape is drawn with currentColor in a muted circular frame,
 * so it scales cleanly and works offline. "" (unset) renders the neutral shape.
 *
 * Distinct silhouettes: `man` (broader, squared shoulders), `woman` (narrower
 * head, flared shoulders), `neutral` (rounded, symmetric).
 */
import type { AvatarKind } from "../api";

type AvatarUser = { displayName?: string; avatarKind?: AvatarKind };

export function Avatar({ user, size = 28 }: { user: AvatarUser; size?: number }) {
  const kind: AvatarKind = user.avatarKind || "neutral";
  const label = user.displayName ? `${user.displayName} avatar` : "avatar";
  return (
    <span
      className="avatar"
      data-testid="avatar"
      data-avatar-kind={kind}
      role="img"
      aria-label={label}
      style={{
        display: "inline-flex",
        width: size,
        height: size,
        borderRadius: "50%",
        background: "var(--avatar-bg, rgba(127,127,127,0.18))",
        color: "var(--avatar-fg, rgba(80,80,90,0.85))",
        flex: "0 0 auto",
        overflow: "hidden",
        verticalAlign: "middle",
      }}
    >
      <svg viewBox="0 0 64 64" width={size} height={size} aria-hidden="true">
        <Silhouette kind={kind} />
      </svg>
    </span>
  );
}

function Silhouette({ kind }: { kind: AvatarKind }) {
  switch (kind) {
    case "man":
      // Broader head, squared-off wide shoulders.
      return (
        <g fill="currentColor">
          <circle cx="32" cy="23" r="13" />
          <path d="M11 64 C11 49 21 42 32 42 C43 42 53 49 53 64 Z" />
        </g>
      );
    case "woman":
      // Narrower head, shoulders flaring to a wider, rounded base.
      return (
        <g fill="currentColor">
          <circle cx="32" cy="22" r="11" />
          <path d="M14 64 C16 48 24 40 32 40 C40 40 48 48 50 64 C44 60 38 59 32 59 C26 59 20 60 14 64 Z" />
          <path d="M32 40 C26 40 18 47 16 64 L48 64 C46 47 38 40 32 40 Z" />
        </g>
      );
    case "neutral":
    default:
      // Rounded, symmetric head + shoulders.
      return (
        <g fill="currentColor">
          <circle cx="32" cy="23" r="12" />
          <path d="M13 64 C13 50 22 43 32 43 C42 43 51 50 51 64 Z" />
        </g>
      );
  }
}
