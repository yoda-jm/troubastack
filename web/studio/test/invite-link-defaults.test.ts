// T122 — the studio's invite-link DEFAULTS: a minted link is single-use and expires in ~a day unless the
// admin widens it. Tests the default-computing SEAM (what an untouched form submits), not the rendering.
import { describe, it, expect } from "vitest";
import {
  inviteInputFromForm,
  INVITE_DEFAULT_EXPIRY_HOURS,
  INVITE_DEFAULT_MAX_USES,
} from "../src/components/InviteLinks";

describe("invite-link defaults (T122)", () => {
  it("the untouched form mints a single-use link that expires in ~a day", () => {
    expect(INVITE_DEFAULT_MAX_USES).toBe("1");
    expect(Number(INVITE_DEFAULT_EXPIRY_HOURS)).toBe(24);
    // the NATURAL act — fill nothing, click Create — now submits BOTH bounds:
    expect(inviteInputFromForm("member", INVITE_DEFAULT_EXPIRY_HOURS, INVITE_DEFAULT_MAX_USES)).toEqual({
      role: "member",
      expiresInHours: 24,
      maxUses: 1,
    });
  });

  it("an admin can still mint a STANDING link by clearing both fields (API zero-value semantics unchanged)", () => {
    expect(inviteInputFromForm("conductor", "", "")).toEqual({ role: "conductor" });
  });

  it("passes through whatever the admin types", () => {
    expect(inviteInputFromForm("member", "72", "5")).toEqual({
      role: "member",
      expiresInHours: 72,
      maxUses: 5,
    });
  });
});
