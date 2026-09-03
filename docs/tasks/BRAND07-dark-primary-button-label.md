# BRAND07 — the dark-mode primary button's label is under the text bar

**Lane:** web-core. **Size:** XS. **Status:** spec, not started.
**Raised by:** the web-core lane itself, in the BRAND03 submission, and flagged rather than quietly
left — which is why it exists as a task instead of a surprise.

## The measurement

In dark mode the submit/primary button paints `#fff` on the `--brand` fill `#e23b9d`:

| | ratio | verdict |
|---|---|---|
| white on `#e23b9d` (today) | **3.93** | under 4.5 — fails for normal text |
| white on `#a5b4fc` (before BRAND03) | **1.99** | effectively unreadable |

**So BRAND03 roughly doubled it and this is not a regression** — it is the last step of an
improvement, left out because BRAND03's scope was "nothing else changes" and a real fix means editing
the button rule. Recomputed independently at review; both figures are exact.

A button label is normal text under WCAG, and this is the **primary action** on forms — the one
control a user must be able to read. 3.93 is close, which is precisely why it will never be noticed
by eye and needs the number.

## Two ways, both measured

**A — keep the fill, darken the label.** The dark-mode `--bg` token is `#100e16`, and

```
#100e16 on #e23b9d = 4.88   ✔
```

This is the recommendation: it clears the bar with margin, **reuses a token that already exists**
rather than inventing a colour, and keeps the button at full brand saturation — it still reads as a
brand-coloured button, which is the point of the whole BRAND series. A dark label on a saturated fill
is also an ordinary, legible pattern, not a workaround.

**B — keep the white label, darken the fill.** `#c72d88` gives 5.05, `#b02679` gives 6.18. Both work,
both cost the button some of its brand colour, and both add a token whose only job is to be a darker
version of one we have.

Pick one deliberately and record why. If A, say in a comment that the label ink is `--bg` **on
purpose**, or the next person will "fix" it back to white.

## Watch for

- **Light mode is not affected** — check before changing anything shared, and do not regress it.
- The **hover/active/disabled** states of the same button. A fix that clears 4.5 at rest and drops
  below it on hover has moved the problem rather than solved it. Measure every state you touch.
- BRAND03's drift guard forbids raw hex in components; this belongs in `styles.css` as a token, like
  everything else.

## Done when

- The dark primary button's label clears **4.5** against its fill, in every interactive state, with
  the measured ratio recorded beside the rule.
- Light mode's ratios are unchanged — state them, do not assume them.
- The BRAND03 drift guard still passes.
