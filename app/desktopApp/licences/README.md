# Licence texts shipped with the desktop apps

- `EPL-2.0.txt` — the Eclipse Public License 2.0, as published by the Eclipse Foundation
  (https://www.eclipse.org/org/documents/epl-2.0/EPL-2.0.txt, fetched 2026-11-06; its wording matches SPDX's
  `EPL-2.0.txt` word for word apart from quote and dash styles). SWT is under it.

WIN08 S2.8: the Windows build copies it, with SWT's own `about.html` and `about_files/` (taken from the SWT jar the
build resolves) and a generated `SWT-NOTICE.txt`, into the Windows app image's `resources/licences/`. Linux and macOS
ship no SWT, so none of it.
