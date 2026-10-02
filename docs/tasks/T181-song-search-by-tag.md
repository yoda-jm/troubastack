# T181 — Searching songs: loose words by default, tags offered, tag chips AND together

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-28, proposing **B5** in the
interaction study (`tags-study`) and approving it after one round of use (*"ok, works for me, spec it"*).

This is the search half that T180 §6 left open. B1–B4 are superseded by B5.

## 1. What already exists

- `BandDetail.tsx` filters the band's songs with **one** folded substring over `title + artist`
  (`foldText(query.trim())` … `.includes(q)`). Tags are unsearchable.
- The filter input only appears above `SONGS_PAGE` (12) songs. **Keep that threshold**; it is not in scope.
- `listSongs` rows already carry `tags`, and this page holds the **whole** list. So everything here is
  client-side, and T181 does **not** depend on T180's `GET /api/bands/{id}/tags`.

## 2. ⟨D1⟩ The box holds words, not one phrase

Split the text on whitespace. **Each word is its own loose term**: folded (`foldText`), substring, over
`title + artist + every tag`. A song matches when it contains **every** word, in any order.

This rule came from a real failure. In the first version of B5, the box was one phrase. VLL typed `ro` and got
3 songs. Then he typed ` op`, meaning to reach the tag `opener`. The suggestion list and the filter both tried
to match `"ro op"` as one string, so both came back empty. **Acceptance must pin this exact sequence (§7).**

## 3. ⟨D2⟩ Tags are suggested for the word being typed, and only that word

The word being typed is the whitespace-delimited run that contains the caret. Offer up to **5** of the
band's tags whose folded form contains that word's folded form. Leave out tags that are already chips. Each
suggestion shows **how many songs the list would show if you picked it**: the current chips, the other
words, and this tag. With nothing else filtering, that is the band-wide count T180 shows. *(Amended
2026-10-02 with ⟨D5⟩: every count on the search surface means the same thing, "this many songs if you pick
it". A band-wide count beside a narrowed list would promise songs the click cannot deliver.)* A suggestion
whose count is **0** still shows, with 0: hiding it would read as "this tag does not exist".

- **Accepting a suggestion** (click, or ↑↓ then Enter) replaces **only the word being typed** with a chip.
  The other words stay in the box as free text, and the caret goes to the end of the box.
- **Enter with no suggestion highlighted does nothing.** It never turns the text into a tag and never picks
  the first suggestion on its own. The box already filters as you type, so Enter has nothing else to do.
- Escape closes the list. Backspace in an **empty** box removes the last chip, and only that one.
- There is **no "create tag" row**. Search cannot create a tag; that belongs to T180.

**Spellings of one tag.** A tag stored under two spellings (`encore` / `Encore`) is **one** suggestion
here. Its label is the most-used spelling, and its count is the number of songs carrying **any** spelling.
Search groups by folded form so the count matches what the chip returns (§4). This is a display-time grouping
only: nothing is renamed or merged in storage, and T180's honest per-spelling cloud is unaffected.

## 3b. ⟨D5⟩ A refine strip under the box: the tags of what is listed (VLL, 2026-10-02)

VLL, after T180 shipped: *"no vocabulary under the search for the tags (at least a few of them that are part
of the current search to refine)"*. Without typing, the reader should **see** which tags would narrow what
they are looking at.

Under the box, one line of clickable tags, **computed from the songs currently listed**:
- each tag carried by **at least one** listed song, with the number of **listed** songs carrying it (the
  ⟨D2⟩ meaning: the songs you get if you pick it);
- **except** tags already chips, and tags carried by **every** listed song. Picking those would not narrow
  anything, and a refine strip that offers a no-op teaches the reader to ignore it;
- most-common first, ties by name; **at most 8**, then **"+N more"** to reveal the rest, the same bounded
  shape as T180's cloud;
- folded spellings grouped, as in ⟨D2⟩;
- a click adds the tag as a chip (strict, AND, §4). The box text is left alone.

**With nothing filtering**, the strip shows the band's most-used tags. It is the entry point for a reader
who has not typed yet, and the reason VLL asked for it. **With no tag left to offer** (everything listed
carries the same tags, or nothing is listed), no strip: an empty line is not information.

It recomputes on every change to the words or chips. It is client-side over the list this page already holds,
so there is no new call.

## 3c. ⟨D6⟩ Tags on the song rows: C1, quiet pills at the right (VLL, 2026-10-02)

VLL asked for the tags on Studio's song list, *"at the right of each item, with a design that works"*, and
picked **C1** from Part C of the interaction study (`tags-study`, three live variants in Studio's own tokens).
This reverses §6's *"Showing tags on the result rows"*, which is struck below.

**The row, on the band's song list:**
- the song's tags as **small pills, right-aligned**, after the title, artist and ✎ badge;
- colour: Studio's **neutral metadata chip** (`--chip-bg` / `--chip-fg`), never the brand and never the
  crimson of `.song-note-badge`. The two must not compete: crimson means "work waiting", grey means
  "description";
- **at most 3** pills, then **"+N"**, which reveals that row's remaining tags in place. Order: **most used
  in the band first**, ties by name. The same tag then sits in the same place row after row, so the column
  can be scanned;
- a pill whose tag is an **active chip** (folded equality, §4) takes the brand-tint style, which is the one
  place colour says "this is filtering";
- **a click toggles the filter**: not a chip → it becomes one (exactly as a §3b strip click); already a chip
  → the chip is removed. The box text is never touched;
- untagged rows show nothing: no empty pill, no placeholder;
- narrow viewport (≈ 400 px): the pills **wrap under the title** on tagged rows only. On a desktop the row
  keeps its height.

**Structure, because the row is a link:** the pills must be **siblings of the `<Link>`, not inside it**. A
`<button>` nested in an `<a>` is invalid, and a pill click would also navigate to the song. The row stays one
visual unit; the link covers the title area.

**Accessibility:** each pill is a `<button>` with an accessible name that says what the click does: *Filter
by "basse"* / *Remove the "basse" filter*. "+N" says *Show N more tags*.

**The box must exist to show the chip.** Today the filter box only renders above `SONGS_PAGE` (12) songs. A
pill click on a smaller band adds a chip, so **while any chip is active the box shows**, whatever the song
count. T182 §3 states the same rule for its panel; one condition serves both.

**Not chosen:** C2 (a second text line, which costs height on a 60-song list) and C3 (fading the other pills
while filtering). C3 was offered as an optional extra; VLL picked C1 alone. Do not add the fade.

## 4. ⟨D3⟩ A chip is strict, and chips AND together

A tag chip keeps a song when one of the song's tags **folds equal** to the chip. That is an exact match on the
whole tag, not a substring: `blues` does not match `slow blues` as a chip, although it does as a word.
**Several chips keep only songs carrying all of them.** OR is not offered (VLL's ruling in the study's
decision table). If OR is ever needed, it has to be a visible control, not a hidden default.

The words in the box then narrow the chip-filtered list further, using the loose rule of §2.

## 5. ⟨D4⟩ The page says what is filtering

- Under the box, one line names the active filter, e.g. *Filtering by tags: slow blues AND encore · text:
  “ro”*. It is empty when nothing filters.
- The empty-result message names the chips too. Today's *No songs match “{query}”* would be wrong when only
  chips are filtering.
- The placeholder becomes **"Filter by title, artist or tag…"**. This T181 decision authorises that label
  change; do not change any other label.
- Changing a chip resets the page limit, as typing does today.

Chips live only in this page's state. They are not written to the URL and are not remembered between visits.
If that is wanted later, it is a separate decision.

## 6. Not in this task

- The Setlists page filter and any song picker elsewhere. This task covers the band song list only.
- ~~Showing tags on the result rows.~~ **Struck 2026-10-02**: now ⟨D6⟩ (C1).
- The setlist page and the add-a-song picker: the C1 row is for the band's song list only, for now.
- Tags on Stage.
- Renaming or merging tags (as in T180 §6).

## 7. Acceptance

Put the matching rules in a **pure module** with unit tests, in the same shape as T180's `tagInput.ts`. Keep
the component thin.

- **VLL's sequence, verbatim.** Use a fixture where `ro` matches 3 songs and `opener` is a tag on one of
  them. `ro` → 3 songs and no suggestion. `ro op` → a suggestion for `opener`, and the list is **exactly** the
  songs that contain both `ro` and `op`, **checked to be non-empty**. An empty list satisfies "every song
  contains both", and the broken first version of B5 returned exactly that. Accepting it → one chip `opener`, box text `ro`, and only songs carrying `opener` that
  contain `ro`. Assert on the **filter output and the box value**, not on rendered text alone.
- Word order does not matter: `op ro` and `ro op` return the same songs.
- A chip is exact. The chip `blues` does not match a song tagged only `slow blues`, and the word `blues` does.
  The fixture must contain such a song, or this line proves nothing.
- Two chips AND. Use a fixture with songs carrying A only, B only, and both: only the song carrying both
  remains. **Assert that the fixture really has all three cases**, so the test fails if the fixture drifts.
- Folded spellings: with `encore` on 2 songs and `Encore` on 1, there is one suggestion with count **3**, and
  the chip returns those 3.
- Enter with no suggestion highlighted leaves the box text and the chips unchanged.
- Backspace in an empty box removes exactly one chip.
- Teeth-check by sabotage, **one side at a time**. (a) Match the whole box as one phrase: the sequence test
  goes red. (b) Suggest from the whole box instead of the word being typed: it goes red again, for a different
  reason. Print the sabotaged line to prove the sabotage landed.
- An e2e on the band page covers the typed sequence end to end, once.
- ⟨D2⟩ count: with one chip active, a suggestion's count equals the number of songs listed after picking it,
  **asserted by picking it and counting**. A band-wide number fails this on a fixture where the tag is on
  more songs than survive the chip; **the fixture must have such a tag**.
- ⟨D5⟩ refine strip: with nothing filtering, it shows the band's top tags (≤ 8, "+N more"). After a chip,
  it shows only tags present in the listed songs, each count equal to the songs listed after clicking it, and
  **omits a tag that every listed song carries**. The fixture needs one such tag, or the omission is
  untested. Clicking a strip tag adds a chip and leaves the box text unchanged.
- ⟨D6⟩ rows: a song with 5 tags shows **exactly 3** pills plus "+2", and the 3 are the band's most used
  (**assert the order** on a fixture where alphabetical and by-use order differ, or the rule is untested).
  "+2" reveals the other two on that row only.
- ⟨D6⟩ a pill click adds the chip and filters the list; a second click on the same tag (pill or chip ×)
  removes it. **A pill click does not navigate**: assert the URL is unchanged.
- ⟨D6⟩ on a band with ≤ 12 songs, a pill click makes the box appear with its chip.
- ⟨D6⟩ the pills are not descendants of the row's `<a>` (a DOM assertion, so a refactor that moves them
  inside the link goes red).
- ⟨D6⟩ an active tag's pill has the active style, and the others do not.
