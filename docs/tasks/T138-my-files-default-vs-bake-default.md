# T138 — "My files" shows every file checked while the bake takes one: two defaults, one state

**Lane:** web-core (studio; no server change needed). **Size:** S. **Status:** spec — found by VLL on the
live demo, 2026-09-04, verified against `e14f5562`.

## What VLL saw

He rebaked a song that has two files (`lyrics`, `bass`) and got **only the lyrics**. In Studio's "my
files" both were **checked**. His question — *"le default du bake n'est pas le même default de cet
affichage ?"* — is the finding: **no, and they disagree.**

## The two defaults

| surface | state: member has NO saved selection | result |
|---|---|---|
| Studio (`MyFileSelection`, `customized=false`) | **ALL pool files**, DisplayOrder order | every box checked |
| baker (no `FileSelection` row) | **ONE file** — lowest DisplayOrder that is a viewable PDF | one file on stage |

Both are individually defensible: Studio's viewer is for *browsing* (show me everything), the stage is for
*performing* (show me my page). **The defect is not that they differ — it is that the UI renders the unset
state identically to a set one.** The only signal is a `custom` pill that is *absent* when nothing is
saved (`Viewer.tsx:1440`). Nobody hunts for a missing pill while reading two ticked boxes.

**T137 made this consequential.** Before it, the bake took one file regardless, so the divergence was
invisible. Now the selection drives what a member reads in performance, and the screen that is supposed to
control that shows something the bake does not honour.

## VLL's shape (2026-09-04), which supersedes my first sketch

**VLL: *"donc aucun ne sera checké, et myfiles aura un warning pour dire lequel sera pris ?"*** Yes —
and this is better than what I first wrote, for a reason worth stating: **the checkbox describes the
SELECTION, not what the viewer displays.** "No selection" should therefore read as *nothing ticked*. My
first sketch kept every box ticked and only added a notice, which preserves the exact display that misled
him in the first place.

So: **unset ⇒ no box ticked**, plus a notice naming the file the stage will take.

### ⚠ The trap this introduces, and its guard

`SetMyFileSelection` accepts an **empty list** as a valid saved selection — *"the member chose to show
nothing"* (`service.go:1591`), stored with `customized=true`. So with nothing ticked by default, a member
who opens the editor and presses **Save** without touching anything goes from *"I see everything in
Studio and the default file on stage"* to **"I see nothing, anywhere"** — in one click, silently,
including in performance.

**Two guards, both needed:**
1. **Save is disabled while nothing has changed.** Covers the accidental click; there is genuinely nothing
   to save from an untouched unset state.
2. **Saving an EMPTY selection asks for confirmation, naming the consequence** — *"You will see no files
   for this song, including on stage."* Covers the deliberate case, which is legitimate but is
   systematically underestimated. This applies from a set state too, where it is already reachable today.

## Fix: say what the stage will do, at the point of decision

Do **not** unify the two defaults — that would either bloat every bundle (bake all files) or lie about
Studio's viewer (show one). Instead, when `customized=false`, the my-files editor states it plainly and
names the consequence:

> **No personal selection.** Studio shows every file; the stage will show **`lyrics`**.
> *Save a selection to choose what you read on stage.*

Naming the file the baker would actually pick is the part that matters — it turns an invisible default
into a visible one. The moment the member saves, both surfaces agree because the choice is explicit.

**The rule this instance belongs to:** the reassurance is displayed where the author stands, the
consequence lands where the reader stands. Same shape as the invite QR blurred beside a legible URL, and
as the tab block whose "left as written" note lived in a form the stage reader never sees.

## Acceptance

- With no saved selection, **no checkbox is ticked**, and the editor names the file the stage will show.
- **Save is disabled** until something changes.
- **Saving an empty selection is confirmed**, with the consequence named — and a test covers the one-click
  path from an untouched unset editor, which is the accident this design makes possible.
- Saving any selection removes that notice and the `custom` pill appears.
- **A test pins the two defaults together**: given a song whose pool is [lower-DisplayOrder viewable PDF,
  another file] and no selection, assert Studio's default view is the whole pool **and** that the notice
  names exactly the file `defaultBakeFile`-style resolution picks. If either default is ever changed, this
  test is what says so.
- No server change: `customized` already carries the distinction; it is only unused at the point where it
  matters.

## Out of scope

Changing either default. The baker's lowest-DisplayOrder-viewable-PDF rule and Studio's show-everything
default both stay.
