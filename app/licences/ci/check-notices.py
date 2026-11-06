#!/usr/bin/env python3
"""A101 — the independent check of the open-source notices INSIDE a shipped artifact (APK, MSI image, Linux image,
macOS app). It never runs the generator's code: the expected components come from Gradle's own `dependencies` report
(and, for Android, from a separate artifact lookup), and what ships is read from the artifact itself. Prints one
`A101-CHECK <name>-<artifact>: PASS|FAIL — …` line per check and exits 1 if any fails.

  coverage-<a>  every group:artifact:version the report resolves has a components line at that version (report →
                listed); on a desktop image every jar inside it maps to a components line too (shipped → listed).
                A report node is excluded only when Gradle's artifact lookup (--artifacts, not the generator)
                resolves NO file for it — what has no file ships no code. Each exclusion is printed with why it has
                none: constraint (a `(c)` line), bom (-bom), kmp-root (its child is its own platform variant: x → x-jvm
                / -android / -desktop / -awt), redirect (its child is the same artifact under another group, JetBrains →
                AndroidX), aggregate (its child's artifact prefixes its own: desktop-jvm-linux-x64 → desktop-jvm),
                common (Kotlin's -common metadata module), or other — printed too, so nothing is hidden by a rule.
  notices-<a>   every NOTICE file inside a shipped component (scanned here) has its text among the shipped texts; and
                every licence a shipped NOTICE or licence file names (by URL or name) has its text in that component's
                entry, or is the other side of an election the components line states ("A OR B, used under B").
  natives-<a>   every native library inside the artifact belongs to a listed component; one in native/manifest.txt has
                that native entry; SWT's to SWT's own entry; the runtime's own and jpackage's launcher to the Java
                runtime, whose legal/ the image carries.
"""
import argparse
import hashlib
import os
import re
import sys
import zipfile

PLATFORM_SUFFIXES = ("-jvm", "-android", "-desktop", "-androidjvm", "-jvmandroid", "-awt")
LICENCE_NAMES = [  # (regex over lower-cased text, SPDX id)
    (r"mozilla\.org/mpl/2\.0|mozilla public license,? v(ersion|\.)? ?2\.0", "MPL-2.0"),
    (r"apache\.org/licenses/license-2\.0|apache license,? version 2\.0", "Apache-2.0"),
    (r"gnu lesser general public|lgpl", "LGPL-2.1-or-later"),
    (r"eclipse public license", "EPL-2.0"),
    (r"opensource\.org/licenses/mit|\bmit license\b", "MIT"),
]
NATIVE = re.compile(r"\.(so|dll|dylib|jnilib)$")
VERBATIM = re.compile(r"(?i)(^|/)(notice|licen[cs]e|copying|copyright)[^/]*$")
NOTICE = re.compile(r"(?i)(^|/)notice[^/]*$")


def parse_report(path):
    """Resolved nodes (g, a, v) and each node's children, from `gradle dependencies` text."""
    nodes, children, constraints = set(), {}, set()
    stack = []
    for line in open(path, encoding="utf-8", errors="replace"):
        m = re.match(r"^((?:[| ] {4})*)[+\\]--- (.*)$", line.rstrip("\n"))
        if not m:
            continue
        depth = len(m.group(1)) // 5
        spec = m.group(2)
        if spec.startswith("project "):
            node = None
        else:
            s = re.match(r"([^\s:]+):([^\s:]+)(?::(\S+))?(.*)$", spec)
            if not s:
                continue
            g, a, v, rest = s.groups()
            arrow = re.search(r"-> (\S+)", rest)
            ver = arrow.group(1) if arrow else (v or "")
            ver = re.sub(r"^\{strictly (.*)\}$", r"\1", ver)
            node = (g, a, ver)
            if "(c)" in rest:
                constraints.add(node)
                node = None
            else:
                nodes.add(node)
        del stack[depth:]
        if node and stack and stack[-1]:
            children.setdefault(stack[-1], set()).add(node)
        stack.append(node)
    return nodes, children, constraints


def why_no_artifact(n, children):
    """Why a report node resolved no file (for the printout; the exclusion itself is 'no file')."""
    g, a, v = n
    kids = children.get(n, set())
    if a.endswith("-bom"):
        return "bom"
    if any(k[1] in tuple(a + s for s in PLATFORM_SUFFIXES) and k[2] == v for k in kids):
        return "kmp-root"
    if any(k[1] == a and k[0] != g for k in kids):
        return "redirect"
    if a.endswith("-common"):
        return "common"
    if any(a.startswith(k[1] + "-") for k in kids):
        return "aggregate"
    return "other"


def read_index(notices_dir):
    entries, component_entry, components, carriers, docs, delegated = {}, {}, {}, {}, {}, {}
    current = None
    for line in open(os.path.join(notices_dir, "index.txt"), encoding="utf-8"):
        f = line.rstrip("\n").split("\t")
        if not f or f[0].startswith("#") or not f[0]:
            continue
        if f[0] == "entry":
            current = f[1]; entries[current] = True; docs[current] = []
        elif f[0] == "component":
            gav = tuple(f[1].split(":"))
            components[gav] = {"ids": set(filter(None, f[2].split(","))), "summary": f[3], "entry": current}
        elif f[0] == "carrier":
            carriers.setdefault(current, set()).add(tuple(f[1].split(":")))
        elif f[0] == "doc":
            docs[current].append(f[2])
        elif f[0] == "delegated":
            delegated[tuple(f[1].split(":"))] = f[3]
    texts = {}
    tdir = os.path.join(notices_dir, "texts")
    for name in os.listdir(tdir):
        texts[name[:-4]] = open(os.path.join(tdir, name), "rb").read()
    return entries, components, carriers, docs, delegated, texts


def strip_hash(jar):
    """An image jar's name without the content hash jpackage adds (hex, leading zeros dropped) and .jar."""
    return re.sub(r"-[0-9a-f]{20,32}$", "", jar[:-4])


def scan_zip(path):
    """(verbatim files {name: bytes}, natives [name]) inside a jar or AAR (and an AAR's classes.jar)."""
    verbatim, natives = {}, []
    try:
        with zipfile.ZipFile(path) as z:
            for n in z.namelist():
                if n.endswith("/") or n.endswith(".class"):
                    continue
                if VERBATIM.search(n):
                    verbatim[n] = z.read(n)
                if NATIVE.search(n):
                    natives.append(n)
                if n == "classes.jar":
                    import io
                    with zipfile.ZipFile(io.BytesIO(z.read(n))) as inner:
                        for m in inner.namelist():
                            if not m.endswith("/") and not m.endswith(".class") and VERBATIM.search(m):
                                verbatim["classes.jar!" + m] = inner.read(m)
    except zipfile.BadZipFile:
        pass
    return verbatim, natives


def mark(name, ok, detail, results):
    results.append(ok)
    print(f"A101-CHECK {name}: {'PASS' if ok else 'FAIL'} — {detail}")


def main():
    sys.stdout.reconfigure(encoding="utf-8")   # the Windows runner's console is cp1252, which has no →
    ap = argparse.ArgumentParser()
    ap.add_argument("--name", required=True)
    ap.add_argument("--report", required=True)
    ap.add_argument("--notices", required=True, help="the shipped notices dir (index.txt + texts/)")
    ap.add_argument("--spdx", required=True, help="app/licences/spdx")
    ap.add_argument("--manifest", required=True, help="app/licences/native/manifest.txt")
    ap.add_argument("--image-jars", help="desktop: the image's app/ folder of jars")
    ap.add_argument("--image-root", help="desktop: the image root (runtime natives, launcher, legal/)")
    ap.add_argument("--artifacts", required=True, help="'g:a:v<TAB>path' lines from the artifact lookup (artifact-paths.init.gradle.kts)")
    ap.add_argument("--apk", help="Android: the APK")
    ap.add_argument("--swt-licences", help="Windows: the image's resources/licences (SWT's own entry)")
    args = ap.parse_args()
    results = []
    entries, components, carriers, docs, delegated, texts = read_index(args.notices)
    listed = set(components) | set(delegated)

    # ---- coverage: report → listed (exact version), and on a desktop image shipped jar → listed.
    nodes, children, constraints = parse_report(args.report)
    with_file = set()
    for line in open(args.artifacts, encoding="utf-8"):
        f = line.rstrip("\n").split("\t")
        if len(f) == 2:
            with_file.add(tuple(f[0].split(":")))
    expected = sorted((nodes & with_file) | (with_file - nodes))
    groups = {}
    for n in sorted(nodes - with_file):
        groups.setdefault(why_no_artifact(n, children), []).append(":".join(n))
    missing = [":".join(n) for n in expected if n not in listed]
    not_in_report = [":".join(n) for n in sorted(with_file - nodes)]
    detail = (f"report → listed: {len(expected)} expected (report nodes with a file), {len(missing)} missing {missing}; "
              f"files not in the report {not_in_report}; excluded (no file) "
              + "; ".join(f"{k}={len(v)} {v}" for k, v in sorted(groups.items()))
              + f"; constraint={len(constraints)}")
    ok = not missing
    if args.image_jars:
        want = {}
        for (g, a, v) in listed:
            want[f"{a}-{v}"] = want.get(f"{a}-{v}", 0) + 1
        unmatched, project = [], []
        for jar in sorted(os.listdir(args.image_jars)):
            if not jar.endswith(".jar"):
                continue
            base = strip_hash(jar)
            if re.match(r"^(desktopApp|shared-desktop)$", base):
                project.append(jar); continue
            if want.get(base, 0) > 0:
                want[base] -= 1
            else:
                unmatched.append(jar)
        detail += f" | shipped → listed: {sum(1 for j in os.listdir(args.image_jars) if j.endswith('.jar'))} jars, project jars {project}, unmatched {unmatched}"
        ok = ok and not unmatched
    mark(f"coverage-{args.name}", ok, detail, results)

    # The shipped components' files, for the scans below: (gav, path).
    shipped = []
    if not args.image_jars:
        for line in open(args.artifacts, encoding="utf-8"):
            f = line.rstrip("\n").split("\t")
            if len(f) == 2:
                shipped.append((tuple(f[0].split(":")), f[1]))
    if args.image_jars:
        by_name = {}
        for gav in listed:
            by_name.setdefault(f"{gav[1]}-{gav[2]}", []).append(gav)
        for jar in sorted(os.listdir(args.image_jars)):
            if jar.endswith(".jar"):
                cands = by_name.get(strip_hash(jar), [None])
                shipped.append((cands[0] if len(cands) == 1 else ("?", strip_hash(jar), ""), os.path.join(args.image_jars, jar)))

    # ---- notices: each NOTICE text ships; each licence a shipped file names has its text in that entry.
    shipped_texts = set(texts.values())
    spdx = {f[:-4]: open(os.path.join(args.spdx, f), "rb").read() for f in os.listdir(args.spdx)}
    notice_missing, named, unnamed = [], [], []
    scanned = {}
    for gav, path in shipped:
        verbatim, natives = scan_zip(path)
        scanned[path] = (gav, natives)
        comp = components.get(gav) if gav else None
        for name, content in verbatim.items():
            if NOTICE.search(name) and content not in shipped_texts:
                notice_missing.append(f"{':'.join(gav or ('?',))}: {name}")
            low = content.decode("utf-8", "replace").lower()
            for rx, lid in LICENCE_NAMES:
                if not re.search(rx, low):
                    continue
                who = ":".join(gav[:2]) if gav else os.path.basename(path)
                if comp is None:
                    if gav in delegated:
                        named.append(f"{who}: {name} → {lid} (delegated: {delegated[gav]})")
                    continue
                entry_texts = [texts.get(t) for t in docs.get(comp["entry"], [])]
                if lid in comp["ids"] and spdx.get(lid) in entry_texts:
                    named.append(f"{who}: {name} → {lid}")
                elif " OR " in comp["summary"] and re.search(re.escape(lid.split("-")[0]), comp["summary"], re.I):
                    named.append(f"{who}: {name} → {lid} (election: {comp['summary']})")
                else:
                    unnamed.append(f"{who}: {name} names {lid}, not in its entry")
    mark(f"notices-{args.name}", not notice_missing and not unnamed,
         f"NOTICE files missing from the shipped texts: {notice_missing}; licences named by shipped files: {named}; "
         f"named but without their text: {unnamed}", results)

    # ---- natives.
    manifest = []
    for line in open(args.manifest, encoding="utf-8"):
        if line.strip() and not line.startswith("#"):
            f = line.rstrip("\n").split("\t")
            manifest.append((f[0], f[2]))
    def native_entry(ga):
        for pat, entry in manifest:
            if (pat.endswith("*") and ga.startswith(pat[:-1])) or ga == pat:
                return entry
        return None
    mapped, unmapped = [], []
    def owner_ok(gav, what):
        if gav is None or gav not in listed:
            unmapped.append(f"{what} (owner {':'.join(gav) if gav else '?'} not listed)"); return
        ga = f"{gav[0]}:{gav[1]}"
        if gav in delegated:
            if args.swt_licences and os.path.isfile(os.path.join(args.swt_licences, "SWT-NOTICE.txt")):
                mapped.append(f"{what} → SWT's entry"); return
            unmapped.append(f"{what} → SWT's entry, but no SWT-NOTICE.txt"); return
        ne = native_entry(ga)
        if ne:
            if gav in carriers.get(ne, set()):
                mapped.append(f"{what} → {ne}")
            else:
                unmapped.append(f"{what} → {ne}, but that entry doesn't name {ga} as its carrier")
        else:
            mapped.append(f"{what} → {ga}'s entry")
    if args.apk:
        so_owner = {}
        for path, (gav, natives) in scanned.items():
            for n in natives:
                so_owner.setdefault(os.path.basename(n), gav)
        with zipfile.ZipFile(args.apk) as z:
            for n in sorted({os.path.basename(x) for x in z.namelist() if x.startswith("lib/") and NATIVE.search(x)}):
                owner_ok(so_owner.get(n), n)
    for path, (gav, natives) in scanned.items():
        if args.image_jars and natives:
            names = sorted({os.path.basename(n) for n in natives})
            owner_ok(gav, f"{os.path.basename(path)} ({', '.join(names) if len(names) <= 8 else f'{len(names)} natives'})")
    if args.image_root:
        # Every native file in the image, wherever it is. In the app folder (beside the jars): the Skiko library
        # Compose extracts from its runtime jar at packaging time → that runtime jar's component. Under a runtime folder,
        # and jpackage's libapplauncher → the Java runtime. Anything else → unmapped.
        app_dir = os.path.realpath(args.image_jars) if args.image_jars else None
        runtime_natives, launchers = [], []
        listed_ga = {f"{g}:{a}": (g, a, v) for (g, a, v) in listed}
        for dirpath, _, files in os.walk(args.image_root):
            here = os.path.realpath(dirpath)
            in_app = app_dir is not None and (here == app_dir or here.startswith(app_dir + os.sep))
            for f in sorted(files):
                rel = os.path.relpath(os.path.join(dirpath, f), args.image_root).replace(os.sep, "/")
                if re.search(r"(^|/)(bin|MacOS)/TroubaStage[^/.]*$|^TroubaStage\.exe$", rel):
                    launchers.append(rel)
                if not NATIVE.search(f):
                    continue
                if in_app:
                    m = re.match(r"^(?:lib)?skiko-(windows|linux|macos)-(x64|arm64)\.(?:so|dylib|dll)$", f)
                    ga = f"org.jetbrains.skiko:skiko-awt-runtime-{m.group(1)}-{m.group(2)}" if m else None
                    if ga:
                        owner_ok(listed_ga.get(ga), f"{rel} (extracted from {ga})")
                    else:
                        unmapped.append(f"{rel} (a native beside the jars, no rule maps it)")
                elif re.search(r"(^|/)runtime[^/]*/", rel) or f.startswith("libapplauncher."):
                    runtime_natives.append(rel)
                else:
                    unmapped.append(f"{rel} (a native outside the app and the runtime)")
        legal = [os.path.join(d, f) for d, _, fs in os.walk(args.image_root) if "/legal" in d.replace(os.sep, "/") for f in fs]
        gpl = any(b"GNU General Public License" in open(p, "rb").read() for p in legal if os.path.isfile(p))
        ok_rt = gpl and len(legal) > 0
        apploader = [r for r in runtime_natives if os.path.basename(r).startswith("libapplauncher.")]
        (mapped if ok_rt else unmapped).append(
            f"{len(runtime_natives)} runtime natives (jpackage's {apploader or 'none'}) and the launchers {launchers} → "
            f"the Java runtime (its legal/: {len(legal)} files, GPL text {gpl})")
    mark(f"natives-{args.name}", not unmapped, f"mapped: {mapped}; unmapped: {unmapped}", results)
    sys.exit(0 if all(results) else 1)


if __name__ == "__main__":
    main()
