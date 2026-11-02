# WIN07 — Windows desktop tests: no shared Skiko unpack

Owner: **web-core**. Spec: Fable, 2026-11-02, per VLL ("ok, fix it"). Size: S.

## The symptom

`desktop (windows)` went red twice on 2026-11-02, both times on commits that changed no code:

| Run | Commit | Job |
|---|---|---|
| 41506618737 | d8412aa5 (docs only) | 125487455443 |
| 41519484287 | 10e723b0 (docs only) | 125537046906 |

- Both failed at the step "desktop tests (incl. DPAPI secret-not-plaintext)", which runs
  `.\gradlew.bat :shared:desktopTest :desktopApp:test`.
- **The failing JVM moves between runs.**
  - On d8412aa5, `:shared:desktopTest` failed: 21 of 674 tests, in the six Skia-touching classes.
  - On 10e723b0, `:desktopApp:test` failed: 2 of 30, both in `DesktopTooltipUiTest`, and `:shared` passed.
- Every failure is `ExceptionInInitializerError` caused by `java.nio.file.AccessDeniedException`. It is anchored at
  the first point where the test starts Skia (`runComposeUiTest {`, `ImageBitmap(…)`) or at `WindowsException.java:89`.
  Later uses of the same class then throw `NoClassDefFoundError`.
- The console log carries no path and no frames: Gradle's short exception format suppresses both, and no test report
  is uploaded.
- **The two test tasks overlap.** In both logs, `:desktopApp:test` and `:shared:desktopTest` run in the same few
  seconds (job 125537046906: 20:41:15–20:41:33; job 125487455443: 19:07:43–19:07:54). The runs store a
  configuration-cache entry, and Gradle then runs tasks in parallel.
- The same tests passed on other runs within minutes.
- Linux and macOS run the same two tasks and have never failed this way.

A flake on `desktop (windows)` holds up the whole release, because `release-apk` needs it.

## The cause (strongly supported; the path is not yet in a log)

Skiko 0.9.22.2 (`org.jetbrains.skiko.Library.findAndLoad`) unpacks its native library on first use. With no
`skiko.library.path` set, it goes to `SkikoProperties.dataPath/<hash>/`, which defaults to `~/.skiko/<hash>/`, a
folder shared by every JVM run as the same user. The unpack works like this:

    if (!file.exists()) { temp = createTempFile(dest); copy resource → temp; Files.move(temp, file, ATOMIC_MOVE) }
    System.load(file)

- On a fresh runner, `~/.skiko` is empty.
- The step runs two test JVMs: one for `:shared:desktopTest` and one for `:desktopApp:test`. Both load Skiko,
  `:desktopApp` through `DesktopTooltipUiTest`.
- With the configuration cache on (`app/gradle.properties`), Gradle may run the two tasks at the same time.
- If both JVMs pass `!file.exists()` before either one moves its copy into place, the first one moves it and
  `System.load`s it, which maps the DLL. The second one's `MoveFileEx(REPLACE_EXISTING)` then lands on a DLL that is
  mapped, and Windows answers ERROR_ACCESS_DENIED, i.e. `AccessDeniedException`.
- POSIX `rename` replaces a mapped `.so` without complaint, which is why Linux and macOS are immune.
- Skiko sets `loaded = true` before it loads. So the losing JVM's first Skia class fails its static init, and every
  Skia-touching test in that JVM fails with it. `:shared:desktopTest` has 22 such tests; 21 failed.

## The fix

Every desktop `Test` task gets its **own** Skiko data directory under its build directory:
`systemProperty("skiko.data.path", <buildDir>/skiko)`. This applies to `:shared`'s desktop test task and to
`:desktopApp`'s `test`.

- It works the same way the desktop tests already isolate `TROUBASTAGE_HOME` (`app/desktopApp/build.gradle.kts:50`).
- Two JVMs then never unpack into the same folder, whatever Gradle runs in parallel.
- Do not serialise the tasks, retry the tests, or catch the exception. None of those removes the shared folder.
- If a task ever uses `maxParallelForks > 1`, the directory must be per fork. Today both tasks run one fork; say so
  in a comment.

## Acceptance

1. **Full exception output.** Both desktop `Test` tasks print failures with
   `testLogging { exceptionFormat = FULL }` (message, cause chain and frames), so the next failure of any kind
   carries its path.
2. **A positive control on Windows.** On a throwaway branch (never landed), a workflow step runs this 8 times on
   `windows-latest`:
   - remove `~\.skiko` and both build `skiko` directories;
   - then `.\gradlew.bat :shared:desktopTest --rerun :desktopApp:test --rerun`.

   Run it once on main's code, with criterion 1 added so that a red shows the denied path and the frames, and once
   with the fix. Report the red/green count of each.
   - Main red at least once, with the path under `.skiko\<hash>\` and frames through `Library.unpackIfNeeded` →
     `Files.move`, confirms the race.
   - The fix must be 8 of 8 green.
   - If main never goes red, say so plainly. The fix still lands on criterion 3, but the diagnosis stays
     unconfirmed.
3. **A guard that stays in CI.** Right after the desktop test step, on all three OSes, assert that `.skiko` does not
   exist under the **JVM's** `user.home`, then list each build's `skiko/<hash>/` contents into the log.
   - On Windows that is `%USERPROFILE%`.
   - On Unix the JVM reads the passwd entry, not `$HOME`, and this job overrides `HOME`. So take the path from the
     JVM, or check both.
   - Sabotage: remove the property from one project, and the guard goes red.
4. A PR run green on every job, `desktop (windows)` included.
