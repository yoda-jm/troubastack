# T141 — A file's `Size` must describe the STORED bytes, not the source it was rendered from

**Lane:** web-core (core). **Size:** S. **Status:** fixed 2026-09-05 (web-core) — import now sets `Size`
from the stored (rendered) blob, and `downloadFile` derives `Content-Length` from `len(data)`, never the
stored field. Two RED-first teeth-checked tests (`TestImport_GeneratedChartSizeMatchesStoredBlob`,
`TestDownloadFile_ContentLengthMatchesBody`). The download defence fixes the symptom even for the 87
existing wrong-size rows; their `Size` field is corrected on the next re-import (freeze-pending). Awaiting
reviewer re-verify. **Severity: HIGH — every generated chart is unviewable in Studio after an import.**

## Symptom

VLL, after his first rehearsal: *"j'ai dans studio (web) actuellement un failed to fetch sur tous les
morceaux altoband, alors que le bake marche."*

## Proof

`GET /api/files/{id}` over loopback, the public host, and the LAN address — all three identical:

```
< HTTP/1.1 200 OK
< Content-Length: 1720
curl: (18) end of response with 1720 bytes missing        exit=18, 0 bytes written
```

**It fails on `127.0.0.1` too**, so it is not the network, not the external IP, not NAT — it is the
server. And the numbers name the cause:

```
file record size : 1720   ← the length of the chart SOURCE
stored blob      : 3029   ← the rendered PDF
```

`downloadFile` sets `Content-Length: f.Size` (1720) and then writes the blob (3029). Go's server refuses
to write past the declared length, the response is malformed, and the browser reports the generic
`TypeError: Failed to fetch`.

**87 of 158 files are affected — and of the 87 that have a chart source, 87 have `size` exactly equal to
the SOURCE length.** Not a coincidence: it is every generated chart, in both bands.

**The bake is unaffected** because it reads the blob directly and never consults `Size` — which is why
VLL saw a working bake and a broken viewer at the same time.

## Root cause

⟨F1⟩ made a folder store a generated chart's **source** bytes, rendered on import. The importer takes
`Size` from the manifest — the source's length — while storing the **rendered PDF** as the blob. The two
fields then describe different objects.

## Fix — two changes, both worth having

1. **Import sets `Size` from the bytes it actually stores.** The manifest's `size` describes the folder
   entry; once the importer renders, only the rendered length is true of the blob.
2. **`downloadFile` derives `Content-Length` from `len(data)`, not from `f.Size`.** It is holding the
   bytes; a stored field is a claim about them, and a claim that disagrees with the payload should never
   be able to corrupt a response. This is the defence that would have contained bug #1 to a wrong number
   in a listing instead of a broken viewer.

## Acceptance — RED FIRST (VLL)

- A test that imports a folder with a generated chart and asserts **`file.Size == len(stored blob)`**.
  Run it before the fix and **see it fail**; a fixture whose source happens to be the same length as its
  render would pass either way, so pick content where they differ (they always do).
- An HTTP test that `GET /api/files/{id}` returns a body **whose length equals its `Content-Length`** —
  the assertion that actually reproduces the symptom. Today it fails; after the fix it passes.
- Repair the existing rows: the live server has 87 wrong sizes, so the fix needs a one-shot correction
  pass (recompute `Size` from the blob) or a re-import once the importer is right.

## Why this one is worth remembering

Two fields described the same file and disagreed, and the layer in between trusted the wrong one. Nothing
was missing — the blob was intact the whole time, and every check that looked at *content* said the data
was fine. Only a check that compared **the declared length against the payload** could see it.
