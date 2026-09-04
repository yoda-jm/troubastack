package domain

// Canonical wire strings for the Zone/Access/Scope enums, matching the HTTP
// annotation API's encoding. ObjectType has its own generated pair
// (objecttype_gen.go, from proto); these are hand-written because Zone/Access/Scope
// are not part of the mirrored object.proto surface. httpapi delegates here so the
// `.tband` v2 export (app package, which cannot import httpapi) and the HTTP edge
// share one mapping and cannot drift. Unknown/unspecified → "" on the way out; an
// unknown string → the enum's zero on the way in.

func ZoneToString(z Zone) string {
	switch z {
	case ZoneConductor:
		return "conductor"
	case ZoneShared:
		return "shared"
	case ZonePersonal:
		return "personal"
	default:
		return ""
	}
}

func ZoneFromString(s string) Zone {
	switch s {
	case "conductor":
		return ZoneConductor
	case "shared":
		return ZoneShared
	case "personal":
		return ZonePersonal
	default:
		return ZoneUnspecified
	}
}

// AccessToString maps AccessRO → "ro" and everything else (incl. the unspecified
// zero) → "rw", preserving the HTTP API's long-standing default.
func AccessToString(a Access) string {
	switch a {
	case AccessRO:
		return "ro"
	default:
		return "rw"
	}
}

func AccessFromString(s string) Access {
	switch s {
	case "ro":
		return AccessRO
	default:
		return AccessRW
	}
}

func ScopeToString(sc Scope) string {
	switch sc {
	case ScopePersonal:
		return "personal"
	case ScopePart:
		return "part"
	case ScopeAll:
		return "all"
	default:
		return ""
	}
}

func ScopeFromString(s string) Scope {
	switch s {
	case "personal":
		return ScopePersonal
	case "part":
		return ScopePart
	case "all":
		return ScopeAll
	default:
		return ScopeUnspecified
	}
}
