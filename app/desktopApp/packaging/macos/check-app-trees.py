"""DESK01 D3 — the universal app's merge rule: the arm64 and x86_64 builds' Contents/app trees must be the same app.

The universal TroubaStage.app keeps ONE app/ folder (jars, resources, both Skiko runtimes) and TWO runtimes. That is
only sound if the two builds' app/ trees hold the same program, so this compares them and fails on any difference it
cannot explain:
  - jars: Compose names them `<name>-<content hash>.jar`, and Gradle's jars are not byte-reproducible, so jars are
    matched by name without the hash and compared entry by entry (name, CRC, size). The one known exception: native
    libraries inside a jar (JNA's `*.jnilib`) are re-signed by each build, so for those only the set of names must match;
  - native libraries Compose extracted into app/ (the Skiko dylibs, re-signed by each build): each the same
    architectures (`lipo -archs`) where both builds have it, and their `.sha256` siblings follow them. The one named
    exception: a build may carry only its own arch's `libskiko-macos-<arch>.dylib`; the universal app keeps both;
  - the launcher cfg: equal once the jars' hash suffixes are removed;
  - anything else: byte-equal.

Usage: check-app-trees.py <arm64 Contents/app> <x86_64 Contents/app>. Exit 0 = the same app; 1 = a difference (listed).
"""
import os
import re
import subprocess
import sys
import zipfile

HASHED_JAR = re.compile(r"-[0-9a-f]{20,40}\.jar$")
SKIKO_NATIVE = re.compile(r"^libskiko-macos-(arm64|x64)\.dylib(\.sha256)?$")


def pairs(a_root, b_root):
    """Files of the two trees paired up: by exact name first (library jars keep the same content-hash name), then a
    leftover jar by its name without the hash, when exactly one such jar is left on each side (a jar built here, whose
    bytes differ between builds). Returns (pairs, unpaired names)."""
    a, b = set(os.listdir(a_root)), set(os.listdir(b_root))
    paired = [(n, n) for n in sorted(a & b)]
    left_a, left_b = a - b, b - a
    for base in sorted({HASHED_JAR.sub(".jar", n) for n in left_a | left_b}):
        ca = [n for n in left_a if HASHED_JAR.sub(".jar", n) == base and n.endswith(".jar")]
        cb = [n for n in left_b if HASHED_JAR.sub(".jar", n) == base and n.endswith(".jar")]
        if len(ca) == 1 and len(cb) == 1:
            paired.append((ca[0], cb[0]))
            left_a.discard(ca[0]); left_b.discard(cb[0])
    return paired, [(n, "arm64") for n in sorted(left_a)] + [(n, "x86_64") for n in sorted(left_b)]


def archs(path):
    return subprocess.run(["lipo", "-archs", path], capture_output=True, text=True).stdout.split()


def jar_entries(path):
    out = {}
    for i in zipfile.ZipFile(path).infolist():
        native = i.filename.endswith((".jnilib", ".dylib"))
        out[i.filename] = None if native else (i.CRC, i.file_size)
    return out


def main(a_root, b_root):
    paired, unpaired = pairs(a_root, b_root)
    names_a = set(os.listdir(a_root))
    problems = []
    for name, side in unpaired:
        if SKIKO_NATIVE.match(name):
            continue  # each build extracts its own Skiko library; the universal app keeps both (union)
        problems.append(f"only in {side}: {name}")
    for name, name_b in paired:
        pa, pb = os.path.join(a_root, name), os.path.join(b_root, name_b)
        if name.endswith(".jar"):
            ea, eb = jar_entries(pa), jar_entries(pb)
            if ea != eb:
                diff = sorted(n for n in set(ea) | set(eb) if ea.get(n, "absent") != eb.get(n, "absent"))
                problems.append(f"jar {name}: {len(diff)} entries differ, e.g. {diff[:3]}")
        elif name.endswith((".dylib", ".jnilib")):
            if archs(pa) != archs(pb):
                problems.append(f"native {name}: {archs(pa)} vs {archs(pb)}")
        elif name.endswith(".sha256") and os.path.splitext(name)[0] in names_a:
            pass  # the hash of a re-signed native library (above)
        elif name.endswith(".cfg"):
            ca = re.sub(r"-[0-9a-f]{20,40}\.jar", ".jar", open(pa).read())
            cb = re.sub(r"-[0-9a-f]{20,40}\.jar", ".jar", open(pb).read())
            if ca != cb:
                problems.append(f"cfg {name} differs beyond the jars' hashes")
        elif open(pa, "rb").read() != open(pb, "rb").read():
            problems.append(f"file {name} differs")
    for p in problems:
        print("MERGE RULE:", p)
    print(f"app/ trees compared: {len(paired)} paired, {len(problems)} unexplained difference(s)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1], sys.argv[2]))
