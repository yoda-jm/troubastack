package app

import (
	"errors"

	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// T195 review F3 — EAGER re-placing of a chart's marks at save. Projection used to be lazy only: a stored
// linked mark kept its Points as drawn on the render it was linked on, and serve/bake composed the line
// edits since then from LineIdentity.History. That history is bounded (maxLineHistory saves), so a mark
// nobody touched while its chart was saved more often than that lost its character map and drifted. Now
// SaveChartSource re-bases every linked mark of the file onto the new render, as a normal mutation per
// mark that changed, so a live link is never more than one save behind. Serve and bake still project
// (lazily) whatever a save could not re-place — a concurrent edit, an engine error — so nothing is lost.

// MarkStore is the narrow view of the annotation engine a chart save needs (engine.Engine satisfies it).
type MarkStore interface {
	Head(songID string) (domain.Snapshot, error)
	Apply(songID string, m domain.Mutation) (domain.Mutation, error)
}

// WithAnnotationEngine wires the annotation engine so SaveChartSource re-places the linked marks of the
// chart it saves. Call at wiring time, before serving.
func (s *Service) WithAnnotationEngine(m MarkStore) *Service {
	s.marks = m
	return s
}

// rebaseRetries bounds the re-read-and-retry of one mark whose version moved under the save (a concurrent
// edit won the race): the next attempt re-bases the NEW stored object, so the edit is kept.
const rebaseRetries = 2

// rebaseChartMarks re-places, on the render (source, blobHash) just saved, every live mark with a stage-2
// link on a layer of f. Called by saveChartSource with the file's line lock held and the ids already
// carried. Each write goes through the engine's LWW checks (BaseVersion = the version read, Version + 1),
// so a concurrent edit is never overwritten: on ErrStaleVersion the mark is re-read and re-based from the
// edit. Every mark re-placed → the file's T194 Rederived flag is cleared (every link is now measured on a
// render after the re-derivation, so the flag has nothing left to say); any failure → it is kept.
func (s *Service) rebaseChartMarks(actor User, f SongFile, source, blobHash string, anchors []chartpdf.Anchor) {
	if s.marks == nil || f.SongID == "" {
		return
	}
	li, err := s.repo.GetChartLineIdentity(f.ID)
	if err != nil || !li.Describes(source) {
		return
	}
	r := chartpdf.ChartRender{Source: source, Lines: li, Anchors: anchors, Hash: blobHash}
	snap, err := s.marks.Head(f.SongID)
	if err != nil {
		return
	}
	onFile := map[string]bool{}
	for _, l := range snap.Layers {
		if l.FileID == f.ID {
			onFile[l.ID] = true
		}
	}
	allDone := true
	for _, o := range snap.LiveObjects() {
		if !onFile[o.LayerID] || o.Link == nil || o.Link.State == domain.LinkFixed {
			continue
		}
		if !s.rebaseOne(actor, f.SongID, o, r) {
			allDone = false
		}
	}
	if allDone && li.Rederived != nil {
		_ = s.repo.SetChartLineIdentity(f.ID, li.ClearRederived())
	}
}

// rebaseOne writes one mark's re-based form, retrying from the re-read object when it lost an LWW race.
func (s *Service) rebaseOne(actor User, songID string, o domain.Object, r chartpdf.ChartRender) bool {
	for attempt := 0; ; attempt++ {
		next, changed := chartpdf.RebaseOnRender(o, r)
		if !changed {
			return true
		}
		next.Version = o.Version + 1
		m := domain.Mutation{Kind: domain.KindMove, UUID: o.UUID, Object: &next, BaseVersion: o.Version,
			AuthorID: actor.ID, Summary: "re-place a mark after a chart edit (T195)"}
		_, err := s.marks.Apply(songID, m)
		if err == nil {
			return true
		}
		if !errors.Is(err, engine.ErrStaleVersion) || attempt >= rebaseRetries {
			return false
		}
		snap, herr := s.marks.Head(songID)
		if herr != nil {
			return false
		}
		found := false
		for _, cur := range snap.LiveObjects() {
			if cur.UUID == o.UUID {
				o, found = cur, true
			}
		}
		if !found || o.Link == nil {
			return true // deleted, or unlinked by the concurrent edit: nothing left to re-place
		}
	}
}
