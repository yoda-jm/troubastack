# T113 — A LICENSE, and the NOTICE that points at one

**Priority:** normal; can land any time, independent of the rest · **Size:** XS · **Area:** repo root
(Web & Core lane). From the 2026-08-25 project audit, finding C1.

## 1. The problem

`NOTICE` opens with *"The TroubaStack source code is licensed under the repository's LICENSE."*
**There is no LICENSE file.** The repository is public, carries badges, and is therefore — legally —
all rights reserved: nobody may use, fork, or contribute to it, and the NOTICE points at a file that
does not exist.

Ten minutes of work that unblocks every external thing.

## 2. What to build

**(a) A `LICENSE` at the repo root.**

**(b) Make `NOTICE` true** — its reference should resolve, and the CC-BY-4.0 / CC-BY-SA-4.0 demo-chart
attributions it already carries should read correctly alongside the code license.

**(c) Say it once more where people look**: a license line in `README.md`.

## 3. The one input, and the default

**Which license is VLL's call, and it is the only decision here.** Recommendation, and what to build
unless he says otherwise: **Apache-2.0**.

The reasoning is not generic — this repo *already* has a `NOTICE` file carrying third-party
attributions, which is an Apache-2.0 convention specifically (§4(d) of that license is what a NOTICE
file is *for*). The existing file is evidence of the intent; adopting Apache-2.0 makes the structure
that is already there correct, and its patent grant suits a project meant to be self-hosted by
strangers. MIT would also be defensible and simpler, but would leave NOTICE as a file with no license
that references it.

Anything else — GPL/AGPL in particular — is a materially different product decision about whether
someone may run a modified TroubaStack without publishing it, and should be his explicit choice, not a
default. If you believe the answer is copyleft, stop and raise it rather than picking.

## 4. Acceptance criteria

- `LICENSE` exists at the repo root with the full, unmodified text of the chosen license and the
  correct copyright line.
- `NOTICE`'s reference resolves; its third-party attributions still read correctly.
- `README.md` states the license.
- No other file's licensing claims are changed without saying so.

## 5. Out of scope

`CONTRIBUTING.md`, `SECURITY.md`, per-file SPDX headers, relicensing anything under `docs/demo-charts/`
(those attributions are already correct and must not be touched).
