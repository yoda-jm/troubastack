package app

import "strings"

// T204 — channel (console-track) keys. A per-musician track is identified band-wide by a channel key:
//   "voice"         — the one voice channel (every voice part a musician sings shares it)
//   "p:<partId>"    — a personal instrument's own default track
//   "g:<channelId>" — a custom group (a Channel)
// A shared part is NOT a per-musician channel; it keeps its own column (shared:<partId>). The patch key for a
// musician's channel is ch:<musicianId>:<channelKey>. Keys are id-based (never a NAME), so a part or group
// rename never re-keys a patch — the M1 trap (Fable).

// PartChannelKey is the per-musician channel key for a NON-shared part ("" for a shared part). Precedence:
// shared wins (→ ""), then an explicit group, then the voice default, then the part's own track.
func PartChannelKey(p Part) string {
	switch {
	case p.Shared:
		return ""
	case p.ChannelID != "":
		return "g:" + p.ChannelID
	case p.Kind == PartVoice:
		return "voice"
	default:
		return "p:" + p.ID
	}
}

// PatchKey is the per-concert patch key for a musician's channel.
func PatchKey(musicianID, channelKey string) string { return "ch:" + musicianID + ":" + channelKey }

// channelLabel is the header text for a channel key.
func channelLabel(key string, partByID map[string]Part, chanByID map[string]Channel) string {
	switch {
	case key == "voice":
		return "Voice"
	case strings.HasPrefix(key, "p:"):
		if p, ok := partByID[strings.TrimPrefix(key, "p:")]; ok {
			return p.Name
		}
	case strings.HasPrefix(key, "g:"):
		if c, ok := chanByID[strings.TrimPrefix(key, "g:")]; ok {
			return c.Name
		}
	}
	return key
}

// MigratePatchKey maps a legacy T203 patch key to the T204 scheme, IDENTITY-preserving (p:<partId> is by id, so
// a part rename never re-keys): voice:<m>→ch:<m>:voice, inst:<m>:<p>→ch:<m>:p:<p>. ch:* and shared:* (and any
// unknown key) pass through unchanged, so it is idempotent.
func MigratePatchKey(key string) string {
	switch {
	case strings.HasPrefix(key, "voice:"):
		return PatchKey(strings.TrimPrefix(key, "voice:"), "voice")
	case strings.HasPrefix(key, "inst:"):
		rest := strings.TrimPrefix(key, "inst:") // "<musicianId>:<partId>"
		if i := strings.IndexByte(rest, ':'); i >= 0 {
			return PatchKey(rest[:i], "p:"+rest[i+1:])
		}
	}
	return key
}
