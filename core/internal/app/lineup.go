package app

import (
	"fmt"
	"sort"
	"strings"
)

// T203 — the line-up vocabulary (musicians + parts) and the pure channel-key helpers. Musicians and parts are
// band-level collections stored on the Band aggregate; a member auto-appears in the roster (resolved at read)
// until something about them is stored (an order, a stage name, or a line-up reference materializes them).

// --- channel keys (reserved for the patch list + sound sheet; the format the amendment fixed) ---

func VoiceChannelKey(musicianID string) string        { return "voice:" + musicianID }
func InstChannelKey(musicianID, partID string) string { return "inst:" + musicianID + ":" + partID }
func SharedChannelKey(partID string) string           { return "shared:" + partID }

// MusicianView is a roster entry resolved for display: the stored/synthesized Musician plus its effective
// display name and whether the person is a current band member (vs a guest or a former member).
type MusicianView struct {
	Musician
	DisplayName string `json:"displayName"`
	IsMember    bool   `json:"isMember"`
	Former      bool   `json:"former,omitempty"` // had an account/was a member, no longer a current member
}

// AccountMusicianID is the deterministic, idempotent id for an account-linked musician.
func AccountMusicianID(bandID, userID string) string {
	return uuidV5(bandID, "musician:"+userID)
}

// resolveName picks a musician's display name: the stored override/guest name if set, else the live account
// display name, else "" (an orphan). deletedAccount reports an account that no longer resolves.
func (s *Service) resolveMusicianName(m Musician) (name string, deletedAccount bool) {
	if m.UserID != "" {
		u, err := s.repo.GetUser(m.UserID)
		if err != nil {
			return m.Name, true // the account is gone → a guest (M2); show whatever name was stored
		}
		if strings.TrimSpace(m.Name) != "" {
			return m.Name, false // a stage-name override
		}
		return u.DisplayName, false // follows the account live
	}
	return m.Name, false
}

// Musicians lists the band's roster: current members (auto-appearing by their account id), plus every stored
// musician (guests, former members, overrides). Any member for whom nothing is stored is synthesized at read.
// Membership-gated (any member may read the roster).
func (s *Service) Musicians(caller User, bandID string) ([]MusicianView, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]Musician, len(band.Musicians)) // by id
	byUser := make(map[string]Musician)                      // by UserID, to dedupe a member against a stored link
	for _, m := range band.Musicians {
		stored[m.ID] = m
		if m.UserID != "" {
			byUser[m.UserID] = m
		}
	}
	members, err := s.Members(caller, bandID)
	if err != nil {
		return nil, err
	}
	memberUser := make(map[string]bool, len(members))

	views := make([]MusicianView, 0, len(band.Musicians)+len(members))
	seen := map[string]bool{}
	for _, mv := range members {
		memberUser[mv.User.ID] = true
		// A stored record linked to this account (possibly a guest that kept its random id after linking)
		// takes precedence; otherwise synthesize the account musician.
		m, ok := byUser[mv.User.ID]
		if !ok {
			m = Musician{ID: AccountMusicianID(bandID, mv.User.ID), UserID: mv.User.ID}
		}
		name, _ := s.resolveMusicianName(m)
		if name == "" {
			name = mv.User.DisplayName
		}
		views = append(views, MusicianView{Musician: m, DisplayName: name, IsMember: true})
		seen[m.ID] = true
	}
	// Stored musicians that are not a current member: guests, and former members / deleted accounts.
	for _, m := range band.Musicians {
		if seen[m.ID] {
			continue
		}
		name, dead := s.resolveMusicianName(m)
		views = append(views, MusicianView{
			Musician: m, DisplayName: name,
			Former: m.UserID != "" && !dead && !memberUser[m.UserID], // a real account, just not a member now
		})
	}
	sortMusicianViews(views)
	return views, nil
}

func sortMusicianViews(v []MusicianView) {
	ord := func(o int) int {
		if o == 0 {
			return 1 << 30 // unset order sorts after explicit orders (then by name)
		}
		return o
	}
	sort.SliceStable(v, func(i, j int) bool {
		if oi, oj := ord(v[i].Order), ord(v[j].Order); oi != oj {
			return oi < oj
		}
		return strings.ToLower(v[i].DisplayName) < strings.ToLower(v[j].DisplayName)
	})
}

// AddMusician adds a GUEST (no account) to the roster, by name — admin only. Guests get a random stored id
// (a name-derived id would collide for two same-named guests, M2).
func (s *Service) AddMusician(caller User, bandID, name string) (Musician, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Musician{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Musician{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Musician{}, fmt.Errorf("%w: a musician needs a name", ErrInvalidInput)
	}
	m := Musician{ID: s.newID(), Name: name}
	band.Musicians = append(band.Musicians, m)
	if err := s.repo.UpdateBand(band); err != nil {
		return Musician{}, err
	}
	return m, nil
}

// MusicianPatch edits a roster entry (admin): a stage-name/guest Name and the stage Order.
type MusicianPatch struct {
	Name  *string
	Order *int
}

// UpdateMusician edits a roster entry — admin only. Editing a member who was only auto-appearing materializes
// a stored record (so the override persists), keyed by the account id.
func (s *Service) UpdateMusician(caller User, bandID, musicianID string, p MusicianPatch) (Musician, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Musician{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Musician{}, err
	}
	m, idx := findMusician(band.Musicians, musicianID)
	if idx < 0 {
		// Auto-appearing member: materialize it if the id is a known account musician.
		mv, ok := s.memberByMusicianID(caller, bandID, musicianID)
		if !ok {
			return Musician{}, ErrNotFound
		}
		m = Musician{ID: musicianID, UserID: mv.User.ID}
		band.Musicians = append(band.Musicians, m)
		idx = len(band.Musicians) - 1
	}
	if p.Name != nil {
		band.Musicians[idx].Name = strings.TrimSpace(*p.Name)
	}
	if p.Order != nil {
		band.Musicians[idx].Order = *p.Order
	}
	if err := s.repo.UpdateBand(band); err != nil {
		return Musician{}, err
	}
	return band.Musicians[idx], nil
}

// LinkMusician links a GUEST musician to a band member's account — admin only. If that account has no line-up
// yet, the guest keeps its id and simply takes the userId; otherwise the guest's line-up entries are merged
// into the account's musician (union per song) and the guest record deleted (M2).
func (s *Service) LinkMusician(caller User, bandID, guestID, userID string) (Musician, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Musician{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Musician{}, err
	}
	g, gi := findMusician(band.Musicians, guestID)
	if gi < 0 || g.UserID != "" {
		return Musician{}, fmt.Errorf("%w: not a guest musician", ErrInvalidInput)
	}
	acctID := AccountMusicianID(bandID, userID)
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return Musician{}, err
	}
	accountUsed := false
	if _, ai := findMusician(band.Musicians, acctID); ai >= 0 {
		accountUsed = true
	}
	for _, sg := range songs {
		if lineupHas(sg.Lineup, acctID) {
			accountUsed = true
			break
		}
	}
	if !accountUsed {
		// The account is unused: the guest becomes that account, keeping its id + line-ups.
		band.Musicians[gi].UserID = userID
		band.Musicians[gi].Name = "" // follow the account live
		if err := s.repo.UpdateBand(band); err != nil {
			return Musician{}, err
		}
		return band.Musicians[gi], nil
	}
	// The account is in use: rewrite the guest's line-ups onto the account musician, drop the guest.
	for _, sg := range songs {
		if remapped, changed := remapLineupMusician(sg.Lineup, guestID, acctID); changed {
			sg.Lineup = remapped
			if err := s.repo.UpdateSong(sg); err != nil {
				return Musician{}, err
			}
		}
	}
	band.Musicians = append(band.Musicians[:gi], band.Musicians[gi+1:]...)
	if _, ai := findMusician(band.Musicians, acctID); ai < 0 {
		band.Musicians = append(band.Musicians, Musician{ID: acctID, UserID: userID})
	}
	if err := s.repo.UpdateBand(band); err != nil {
		return Musician{}, err
	}
	m, _ := findMusician(band.Musicians, acctID)
	return m, nil
}

// DeleteMusician removes a stored roster record (a guest or a former member) — admin only. A current member
// cannot be removed (they auto-appear). If the musician is referenced by any line-up it is refused unless
// force, which first removes their entries. Returns how many songs referenced them.
func (s *Service) DeleteMusician(caller User, bandID, musicianID string, force bool) (int, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return 0, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return 0, err
	}
	if mv, ok := s.memberByMusicianID(caller, bandID, musicianID); ok {
		_ = mv
		return 0, fmt.Errorf("%w: a current band member cannot be removed from the roster", ErrInvalidInput)
	}
	_, idx := findMusician(band.Musicians, musicianID)
	if idx < 0 {
		return 0, ErrNotFound
	}
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return 0, err
	}
	refs := 0
	for _, sg := range songs {
		if lineupHas(sg.Lineup, musicianID) {
			refs++
		}
	}
	if refs > 0 && !force {
		return refs, fmt.Errorf("%w: musician is in %d song(s)", ErrInvalidInput, refs)
	}
	for _, sg := range songs {
		if !lineupHas(sg.Lineup, musicianID) {
			continue
		}
		kept := make([]LineupEntry, 0, len(sg.Lineup))
		for _, e := range sg.Lineup {
			if e.MusicianID != musicianID {
				kept = append(kept, e)
			}
		}
		sg.Lineup = kept
		if err := s.repo.UpdateSong(sg); err != nil {
			return refs, err
		}
	}
	band.Musicians = append(band.Musicians[:idx], band.Musicians[idx+1:]...)
	return refs, s.repo.UpdateBand(band)
}

// memberByMusicianID finds the current member whose account musician id equals musicianID.
func (s *Service) memberByMusicianID(caller User, bandID, musicianID string) (MemberView, bool) {
	members, err := s.Members(caller, bandID)
	if err != nil {
		return MemberView{}, false
	}
	for _, mv := range members {
		if AccountMusicianID(bandID, mv.User.ID) == musicianID {
			return mv, true
		}
	}
	return MemberView{}, false
}

func findMusician(ms []Musician, id string) (Musician, int) {
	for i, m := range ms {
		if m.ID == id {
			return m, i
		}
	}
	return Musician{}, -1
}

func lineupHas(l []LineupEntry, musicianID string) bool {
	for _, e := range l {
		if e.MusicianID == musicianID {
			return true
		}
	}
	return false
}

// remapLineupMusician rewrites entries from fromID onto toID, unioning parts if toID already has an entry.
func remapLineupMusician(l []LineupEntry, fromID, toID string) ([]LineupEntry, bool) {
	if !lineupHas(l, fromID) {
		return l, false
	}
	out := make([]LineupEntry, 0, len(l))
	var target *LineupEntry
	for i := range l {
		if l[i].MusicianID == toID {
			e := l[i]
			out = append(out, e)
			target = &out[len(out)-1]
		}
	}
	for _, e := range l {
		switch e.MusicianID {
		case toID:
			// already copied
		case fromID:
			if target != nil {
				target.PartIDs = unionStrings(target.PartIDs, e.PartIDs)
				target.Optional = target.Optional && e.Optional
				target.Note = joinNotes(target.Note, e.Note) // keep the guest's note on the merge
			} else {
				e.MusicianID = toID
				out = append(out, e)
				target = &out[len(out)-1]
			}
		default:
			out = append(out, e)
		}
	}
	return out, true
}

// normalizeLineup drops entries with no musician and de-dupes each entry's part ids (keeping order).
func normalizeLineup(l []LineupEntry) []LineupEntry {
	out := make([]LineupEntry, 0, len(l))
	for _, e := range l {
		if strings.TrimSpace(e.MusicianID) == "" {
			continue
		}
		e.PartIDs = unionStrings(e.PartIDs, nil)
		out = append(out, e)
	}
	return out
}

// materializeMemberMusicians stores a roster record for any current-member musician referenced by a line-up
// that is not already stored, so the reference persists if that member later leaves. Best-effort + idempotent.
func (s *Service) materializeMemberMusicians(caller User, bandID string, l []LineupEntry) error {
	members, err := s.Members(caller, bandID)
	if err != nil {
		return nil // can't resolve members → leave the ids as-is (resolved at read)
	}
	acctByID := make(map[string]string, len(members)) // musicianID → userID
	for _, mv := range members {
		acctByID[AccountMusicianID(bandID, mv.User.ID)] = mv.User.ID
	}
	band, err := s.repo.GetBand(bandID)
	if err != nil {
		return err
	}
	changed := false
	for _, e := range l {
		userID, isMember := acctByID[e.MusicianID]
		if !isMember {
			continue
		}
		if _, idx := findMusician(band.Musicians, e.MusicianID); idx < 0 {
			band.Musicians = append(band.Musicians, Musician{ID: e.MusicianID, UserID: userID})
			changed = true
		}
	}
	if changed {
		return s.repo.UpdateBand(band)
	}
	return nil
}

// exportMusicians projects the stored roster to the .tband wire shape, resolving each account musician's
// username at export (so a future cross-server import can re-link by username).
func (s *Service) exportMusicians(ms []Musician) []v2Musician {
	out := make([]v2Musician, 0, len(ms))
	for _, m := range ms {
		v := v2Musician{ID: m.ID, UserID: m.UserID, Name: m.Name, Order: m.Order, Instruments: m.Instruments}
		if m.UserID != "" {
			if u, err := s.repo.GetUser(m.UserID); err == nil {
				v.Username = u.Username
			}
		}
		out = append(out, v)
	}
	return out
}

// importMusicians maps the wire shape back to stored musicians. UserID is kept verbatim; Username is export
// metadata for a future cross-server re-link (the remap is stage 1b), not stored.
func importMusicians(vs []v2Musician) []Musician {
	out := make([]Musician, 0, len(vs))
	for _, v := range vs {
		out = append(out, Musician{ID: v.ID, UserID: v.UserID, Name: v.Name, Order: v.Order, Instruments: v.Instruments})
	}
	return out
}

// joinNotes concatenates two notes with " / ", dropping an empty one.
func joinNotes(a, b string) string {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case a == b:
		return a
	default:
		return a + " / " + b
	}
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
