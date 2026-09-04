# T133 — the confirm dialog's `danger` flag is plumbed through and then silently discarded

**Lane:** web-core. **Size:** XS. **Status:** spec, not started. **Not frozen.**
**Found:** 2026-09-04, while verifying an unrelated claim in the T132 sweep (`7267f206`) — the lane said
`.btn.danger` had no other users, which is true; checking *what else* uses `danger` turned this up.

## The finding

`Dialog.tsx:204` sets the confirm button's class from the flag:

```tsx
className={`${request.kind === "confirm" && request.opts.danger ? "danger" : "primary"} btn-sm`}
```

**But nothing styles `.danger` in a dialog.** The only three rules mentioning it are scoped elsewhere:

| rule | scope |
|---|---|
| `.sel-toolbar button.danger:hover` | the song editor's selection toolbar |
| `.kebab-menu button.danger` | the kebab menu |
| `.row-menu-item.danger` | the concert row menu |

`button.btn-sm` sets only padding and font-size. So the destructive branch renders **identically to the
ordinary one** — and worse than `primary`, since it loses the primary styling it would otherwise have
had.

## Why it matters: five callers ask for it, all genuinely destructive

- `Setlists.tsx:109` — *Delete "«concert»"?*
- `SetlistDetail.tsx:998` — *Delete this setlist?*
- `BandSettings.tsx:144` — *Remove this member?*
- `BandSettings.tsx:158` — *Leave this band?*
- `BandSettings.tsx:364` — **_Delete this band?_ / _"This cannot be undone."_**

Every one passes `danger: true` believing it buys a warning. **The most destructive action in the
product — deleting a band, whose own body says *"This cannot be undone"* — confirms through a button
that looks like "OK".** That sentence and that button are on the same screen, and only one of them is
telling the truth. The dialogs do carry clear titles, so nobody
is unwarned — but the affordance the code explicitly requests is not delivered, and five authors have
now written it in good faith.

## Not caused by the T132 sweep

`7267f206` deleted `.btn.danger`, and this button never matched it (it has `danger btn-sm`, not
`btn danger`). The gap predates it. **The sweep's claim was accurate** — this is a different hole that
looking for other `danger` users exposed.

## Work

Give the dialog's destructive button a real style. **Use `--error`, not `--live`** — this one genuinely
*is* destructive, which is the distinction the whole T132 sweep drew: live is a state, delete is a
danger. Reuse the existing error tokens rather than minting new ones.

## Done when

- The destructive confirm is visually distinct from the ordinary one — asserted in a test, not
  eyeballed: a computed-style check like the T132 dot assertion, since "the class is present" would
  pass on today's broken version.
- Contrast against the dialog surface measured and stated, both themes.
- The five callers are unchanged — the flag already says what they mean; only the styling was missing.
