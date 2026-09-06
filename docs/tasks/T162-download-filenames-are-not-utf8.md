# T162 — a download's filename is mangled when it is not ASCII

**Lane:** core. **Kind:** bug. **Number claimed** in the same push as this file.
Reported by VLL, 2026-09-06, on the setlist export: *"le nom de fichier avec des caractères utf8 n'est pas
utf8, probablement un bug d'encoding."* He is right, and it is not one endpoint.

## What is wrong

Every download sets the header the same way:

```go
w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
```

The bare `filename` parameter is **not** UTF-8. Per RFC 6266 it is an HTTP `quoted-string`, i.e. Latin-1;
a UTF-8 byte sequence dropped into it is undefined, and browsers each mangle it differently — the accented
characters come back as mojibake or the name is truncated at the first non-ASCII byte. The fix is the
`filename*` parameter of RFC 5987: `filename*=UTF-8''<percent-encoded>`, sent **alongside** a plain ASCII
`filename` as the fallback for anything that does not understand it.

## It is five endpoints, not one — the enumeration

`git grep 'Content-Disposition' core/internal/httpapi` (non-test), and **not one uses `filename*`**:

| Site | Filename comes from | User-controlled? |
|---|---|---|
| `webapi.go:1076` | the setlist export — band + setlist name | **yes** (VLL's report) |
| `bandio.go:41` | the band export — band name | **yes** |
| `webapi.go:938` | viewing a file — its stored name | **yes** |
| `bakeapi.go:336`, `:386` | a concert id | no (uuid) |
| `appsapi.go:113` | an app build name + version | no |

The three marked **yes** are the bug. The other two are ASCII by construction and need nothing, but the fix
belongs in **one shared helper** anyway, so a future endpoint cannot get it wrong by being written the
obvious way.

## What to build

One helper — `contentDisposition(name string) string` in `httpapi` — returning
`attachment; filename="<ascii fallback>"; filename*=UTF-8''<percent-encoded>`, and every site uses it.

- **The ASCII fallback must not be empty.** Strip to ASCII; if nothing survives (a fully non-Latin name),
  fall back to a stable generic (`setlist.pdf`, `band.tband`, …) rather than `filename=""`.
- **Percent-encode per RFC 5987**, not `url.QueryEscape` — the latter encodes space as `+`, which is wrong
  here and produces a literal `+` in the saved name.
- **Quote-safety:** a `"` or `\` in a name must not break out of the quoted-string. That is a header
  injection, not only a cosmetic bug — check it.

## ⟨R1⟩ Red first

- A band whose name carries accents → the header contains `filename*=UTF-8''` with the correctly
  percent-encoded name, **and** an ASCII fallback that is not empty. Red today.
- **Teeth:** assert the round-trip — decoding the `filename*` value returns the original string **exactly**.
  Asserting only "the header contains `filename*`" would pass on a wrongly-encoded value.
- A name that is pure ASCII produces a header a strict old client still parses (the plain `filename` first,
  unchanged from today).
- A name containing `"` and `\` and a newline does not escape the quoted-string or inject a header.
- A name with no ASCII characters at all still yields a usable fallback.

## Done means

VLL downloads his running-order sheet and the saved file carries his band's name with its accents intact,
on the browser he actually uses. **That last part is a device/browser check, not a unit test** — the header
can be perfect and a browser still surprise us.
