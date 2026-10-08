# A80 — Fresh Stage screenshots for the project page

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-10-08: the GitHub Pages site is
being redesigned and should show the current state. Its Stage frames predate note mode, the Notes tab,
pedals/Learn and the current chrome.

Sibling: **T187** (Studio screenshots, web-core).

## Hard rules

- **Synthetic demo data only**: the committed demo bundle (`docs/demo/demo-concert.tstage`, *Sat @ The
  Anchor*, The Open Road…) or a bake from a **seeded isolated server**. These go on a public website.
- **VLL's tablet holds real concerts.** Their names appear on Home, in the concert list and in the Notes tab. So
  **either use an emulator** (preferred: same resolution class, 1200×1920 tablet), **or**, on the tablet, make
  sure no frame shows a real concert, band, member or song. Hiding by cropping is not enough if a title can
  still be read anywhere. **Ask VLL before adb**, as always.
- Clean frames: no debug overlays, no pointer-location traces, no toasts unless they are the subject.

## Format

Tablet frames at native resolution (landscape **1920×1200**, portrait **1200×1920**). PNG, to
`docs/screenshots/site-<id>.png`. Commit them, then post the list at the gate. **Fable reviews every frame
before it ships.**

## The shots

| id | What it shows | Notes |
|---|---|---|
| `m1-perform` | **Two-up landscape** in the **warm** or **night** scheme, chrome **visible** (position label, ☰ ✎ ⚙ ✕). | the Stage hero |
| `m2-immersive` | The same spread, chrome hidden: the whole screen is the score. | pair with m1 |
| `m3-note` | **Note mode**: a fresh handwritten note on the page, the note bar at the bottom with **Clear page** and the **✓**. | landscape |
| `m4-notes-tab` | The **Notes tab**: *Send all (N)* in the title row, a concert header, a couple of notes, and the *Sent* group. Demo concert only. | portrait or landscape |
| `m5-learn` | The **pedal Learn** panel in Parameters (A72/A76), mid-learn if possible. | |
| `m6-autoupload` | The **armed auto-upload banner** under the menu (A77). | optional |

## Not in this task

Any UI change; file what looks wrong instead of fixing it here.
