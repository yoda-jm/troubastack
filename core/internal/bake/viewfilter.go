package bake

// View-time layer resolution (P205) — the ONE rule that decides whether a baked
// layer is composited into a viewer's page. It is shared by construction between
// the printed PDF (T57, this package) and the on-stage presenter (P205 Stage 3,
// app/shared): both run the SAME cases from testdata/view-resolution.vectors.json
// so "print == screen" is a tested invariant, not a hope (the glyphs.json pattern
// applied to semantics — see testdata/README.md).
//
// A printed backup must be REPRODUCIBLE, so this rule never consults a live
// session's manual toggles — only bake-time facts (mandatory, role_tag, owner,
// default_on) and the chosen viewer identity (role + member id). The precedence,
// highest first (P205 spec §49 / T57 ruling 2026-07-18):
//
//  1. mandatory        → always on (I12; the viewer can never hide it).
//  2. personal layer   (owner != "") → on IFF it is the viewer's own (owner == memberID).
//     Identity outranks default_on: my own layers print for me.
//  3. shared layer     (owner == "") → role-gated AND default-gated:
//     roleOK   = role_tag == "" (untagged, everyone) OR role_tag == viewer role.
//     defaultOK = default_on present ? *default_on : true (absent ⇒ legacy "on").
//     visible  = roleOK && defaultOK.
//
// viewerRole is the EXPLICIT print role ("" = a fresh viewer: mandatory + untagged
// shared only); viewerMemberID is the identity whose personal layers to include.
func LayerVisible(l LayerImage, viewerRole, viewerMemberID string) bool {
	if l.Mandatory {
		return true
	}
	if l.Owner != "" { // personal layer — identity gate (outranks default_on)
		return l.Owner == viewerMemberID
	}
	// shared/band layer — role gate AND bake-time default gate
	roleOK := l.RoleTag == "" || l.RoleTag == viewerRole
	defaultOK := l.DefaultOn == nil || *l.DefaultOn
	return roleOK && defaultOK
}
