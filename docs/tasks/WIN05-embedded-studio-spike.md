# WIN05 — Studio inside the Windows app: a measured spike, then VLL decides

Owner: **web-core**. Starts after **WIN02** has landed (it needs the desktop window). It is a measurement, not a
feature. **Nothing from it lands except the report.** Spec: Fable, 2026-11-01. Size: S (time-boxed).

## Why (VLL, 2026-11-01)

> "ensuite on fera studio en embarqué si ce n'est pas trop gros"

"Pas trop gros" needs numbers, so this task produces them before anyone builds it.

## What the Android app does (the bar to match)

`EditScreen` hosts Studio in `android.webkit.WebView` through the `WebViewHost` seam:
- it seeds the session cookie before loading;
- it loads `embeddedUrl(server, path)`, and Studio keeps `?embedded=1` in `sessionStorage`;
- it bridges web → app with `@JavascriptInterface` and app → web with `evaluateJavascript`;
- it shows the A93 error cover and Retry;
- ⋮ Reload keeps the current page.

The annotation pen in Studio is the showcase: drawing must feel the same as in a desktop browser.

## The candidates

1. **Chromium embedded** (JCEF, e.g. via the KCEF packaging for Compose).
2. **WebView2** (the Edge runtime that ships with Windows 11), through whatever Java/Kotlin binding exists. Judge
   the binding's maturity honestly.
3. **No embedding**: the app opens Studio in the default browser at the right page. It is listed so its cost (zero)
   and its losses are written down next to the other two.

## Method

A throwaway branch per candidate, never presented for landing. For each embedding candidate, the prototype loads
Studio against a scratch core in the desktop window, with the session seeded, `?embedded=1`, and one bridge round
trip (web → app → web) working.

## The report (posted at the gate)

For each candidate:
- installer size delta (MSI before and after);
- cold start to Studio interactive;
- memory with Studio open;
- whether pen drawing in the editor feels the same as in Edge or Chrome on the same machine (say how you judged it);
- how the cookie, the bridge, error and Retry, and Reload map;
- licence and redistribution terms;
- what breaks on a machine without the runtime (WebView2 on Windows 10);
- a size estimate for the real task.

End with a recommendation. VLL decides; if the answer is go, I write the implementation spec from the report.

## Out of scope

Landing anything; macOS/Linux embedding; changing Studio itself (any Studio change the bridge would need is
reported, not made).
