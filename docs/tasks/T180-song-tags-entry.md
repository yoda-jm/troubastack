# T180 — Tagging a song: suggestions with counts, plus the band's vocabulary as a cloud

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-24, choosing **A2 + A3**
from the interaction study (`tags-study`, eight live proposals).

**The search half is NOT decided.** B1–B4 are still open; this task is entry only, and the two halves were
built to be independent.

## 1. What already exists

- `Song.Tags []string` — storage, wire, band folder and bake all carry it.
- Studio's `SongDetails` writes it from a **comma-separated** text field (`split(",")`, trim, drop empties).
  **Spaces inside a tag already work**; no quoting scheme is needed and none should be introduced.
- The band song search matches **title and artist only** — tags are unsearchable. Out of scope here.

## 2. ⟨D1⟩ The commit gesture is the delimiter

Type freely; **Enter or comma** commits the buffer as one tag, spaces included. Backspace on an empty input
removes the last chip. This is how YouTube behaves and it is why the space-versus-comma debate does not need
settling: there is no delimiter *inside* the text.

Keep comma working so pasting `slow blues, encore, needs work` still does the obvious thing in one gesture.

## 3. ⟨D2⟩ Suggestions carry a usage count (this is the load-bearing half)

As the reader types, offer the band's existing tags with **how many songs already use each**. Below the
matches, a **Create "…"** row for a genuinely new tag.

**The count is the feature, not decoration.** A tag vocabulary fails by fragmenting — `encore`, `Encore`,
`encores` minted by three people in one week — and the only thing that prevents it is making the existing
word the path of least resistance at the moment of typing. Without the count a suggestion list is just
autocomplete; with it, the reader can tell a convention from someone's typo.

Match on a **folded** comparison (case- and accent-insensitive, the existing `foldText`), and if the typed
text folds to an existing tag, **offer that tag rather than creating a second spelling**.

## 4. ⟨D3⟩ The band's tags also sit under the field as a clickable cloud

Toggle to add or remove. It is the fastest path for the common case and it is self-documenting: a new member
sees the band's whole vocabulary without typing.

**It does not scale, so bound it:** show the **most-used N** (start at 12) with a "show all" affordance
rather than rendering forty chips. The cloud serves the head of the distribution; ⟨D2⟩'s typing path serves
the tail. That division is the reason both were chosen instead of one.

## 5. ⟨D4⟩ Where the vocabulary comes from

One band-wide list of `{tag, count}`, fetched once for the editor — **not** N calls, and not derived by the
client from a song list it may not have in full. Same shape as T173's aggregate and for the same reason.

## 6. Not in this task

- **Search.** B1–B4 remain open. Nothing here should presuppose one: do not add a tag chip to the search box.
- **Renaming or merging a tag across a band.** Real, and a different task — it is a write that touches every
  song carrying the word, which is a decision with VLL's name on it.
- **Tags on Stage.** This is a Studio editing surface; the tablet is not in scope.

## 7. Acceptance

- Typing `slow blues` + Enter stores **one** tag containing a space — asserted on the stored array, not the
  rendered chip.
- A tag differing only by case or accent from an existing one offers the **existing** tag; accepting it does
  not create a second spelling.
- The count shown next to a suggestion equals the number of songs carrying it.
- The cloud shows at most N and reveals the rest on request.
- Removing the last chip with Backspace does not also delete the one before it.
- A trailing comma creates **no** empty tag (today's field does; the chip field must not inherit it).
