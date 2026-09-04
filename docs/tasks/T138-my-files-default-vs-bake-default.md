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

- With no saved selection, the my-files editor shows the unset state **and names the file the stage will
  show**; the checkboxes no longer read as a saved choice.
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
