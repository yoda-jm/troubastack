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
suggestion shows the **number of songs carrying the tag, across the band**, the same count T180 shows.

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
- Showing tags on the result rows.
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
