package com.troubastack.desktop

import com.sun.jna.Library
import com.sun.jna.Native
import com.troubastack.shared.seams.desktopDataRoot
import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption

/**
 * A101 — Linux: keep Skia's font fallback away from bitmap fonts. Skiko 0.9.22.2 crashes (SIGSEGV in
 * `SkTypeface_fontconfig::onOpenStream`) when its fallback picks a non-scalable font. On VLL's own machine the only
 * CJK-capable fonts are the misc PCF bitmaps (18x18ja, 18x18ko), so any CJK text — the Java runtime's libpng notice, a
 * song title — crashed the app. fontconfig is pointed at a configuration that includes the person's own and rejects
 * non-scalable fonts. It's a libc `setenv(FONTCONFIG_FILE)` at the very top of main, before AWT or Skia loads a font:
 * the AppImage's AppRun is a symlink to the jpackage launcher (no script to put it in), and in main it holds however the
 * app starts — through FUSE, `--appimage-extract-and-run`, or the image's own `bin/TroubaStage`. Where no scalable CJK
 * font exists the text shows missing-glyph boxes, and stays verbatim.
 *
 * Which file, and its lifetime:
 *  - FONTCONFIG_FILE unset (the usual case): the image's own [SHIPPED] file. Nothing is written or deleted.
 *  - FONTCONFIG_FILE an absolute path that is not a readable file (a mistake, or inherited from an earlier image whose
 *    mount is gone): the image's own file too. Wrapping it would include nothing — no fonts at all — where fontconfig
 *    on its own falls back to its defaults.
 *  - FONTCONFIG_FILE already one of ours (a relaunch inherits it): kept as it is.
 *  - FONTCONFIG_FILE set to the person's own: a wrapper including it, [WRAPPER] in $XDG_RUNTIME_DIR (the person's own,
 *    mode 0700, emptied at logout) or, without one, in the data root. One file, replaced whole at each launch, never
 *    deleted by the app, so no kill or crash leaves another behind.
 * If no file can be used, one line goes to stderr ([WARNING]) and fontconfig is left as it was.
 *
 * The setenv is this process's only: every program the app starts on Linux (xdg-open, for links) gets FONTCONFIG_FILE
 * as it was at launch ([childEnvironment]) — a browser it starts must not run on our file, which may sit in the
 * AppImage's mount and vanish when the app quits.
 */
internal object LinuxFontConfig {
    /** In the Linux image's resources: the system's configuration, minus bitmap fonts. */
    const val SHIPPED = "troubastage-fonts.conf"
    /** The wrapper of a person's own FONTCONFIG_FILE. */
    const val WRAPPER = "troubastage-fonts.conf"
    /**
     * The system's configuration as fontconfig itself finds it: a relative include resolves through FONTCONFIG_PATH and
     * then the system library's own configuration folder (/etc/fonts on Linux distributions), so a person's
     * FONTCONFIG_PATH still applies.
     */
    const val SYSTEM = "fonts.conf"
    /** The start of the stderr line when the fix can't be applied (the Linux CI runs check it is absent). */
    const val WARNING = "TroubaStage: could not set up fonts without bitmap fonts"
    private const val MARKER = "TroubaStage (A101)"

    /** What [apply] did, for the About smoke's log: the file fontconfig runs with (null: left alone), and why. */
    @Volatile var inUse: File? = null
        private set
    @Volatile var how: String = "not applied (not Linux)"
        private set
    /** FONTCONFIG_FILE as it was when the app started (null: unset), read before the setenv. */
    @Volatile private var atLaunch: String? = null
    @Volatile private var readAtLaunch = false

    /** A fontconfig file that includes [include] and rejects every non-scalable (bitmap) font. */
    fun wrapper(include: String): String = """<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<!-- $MARKER: this configuration, minus non-scalable (bitmap) fonts, which Skia's font fallback must not open -->
<fontconfig>
  <include ignore_missing="yes">${xmlEscape(include)}</include>
  <selectfont><rejectfont><pattern><patelt name="scalable"><bool>false</bool></patelt></pattern></rejectfont></selectfont>
</fontconfig>
"""

    /**
     * The FONTCONFIG_FILE to run with, and how it was chosen: [shipped] when the person has none of their own; [own] when
     * it is already one of ours (a relaunch inherits it); else [write]'s wrapper of [own] (of the system's when
     * [shipped] is missing). [write] throws when it can't write; the result is then a failure.
     */
    fun choose(own: String?, shipped: File?, write: (include: String) -> File): Result<Pair<File, String>> = runCatching {
        fun system(why: String) =
            if (shipped != null && shipped.isFile) shipped to "the image's own file ($why)"
            else write(SYSTEM) to "a wrapper of the system's configuration ($why; the image's file is missing)"
        when {
            own.isNullOrBlank() -> system("FONTCONFIG_FILE unset")
            // An absolute path that isn't a readable file: wrapped, it would include nothing (no fonts at all).
            File(own).isAbsolute && !(File(own).isFile && File(own).canRead()) -> system("FONTCONFIG_FILE=$own is not a readable file")
            isOurs(File(own)) -> File(own) to "kept (FONTCONFIG_FILE is already ours)"
            // A relative name is wrapped as it is: fontconfig resolves it through FONTCONFIG_PATH.
            else -> write(own) to "a wrapper of FONTCONFIG_FILE=$own"
        }
    }

    private fun isOurs(f: File) = runCatching { f.useLines { lines -> lines.take(3).any { MARKER in it } } }.getOrDefault(false)

    /** On Linux, point fontconfig at [choose]'s file before any font loads. Sabotage nofontfix (a CI run) skips it. */
    fun apply() {
        if (!isLinux()) return
        atLaunch = System.getenv("FONTCONFIG_FILE"); readAtLaunch = true   // first: before any setenv
        if (System.getenv("A101_SABOTAGE") == "nofontfix") { how = "off (sabotage nofontfix)"; return }
        val runtimeDir = System.getenv("XDG_RUNTIME_DIR")?.let(::File)?.takeIf { it.isDirectory && it.canWrite() }
        val dir = runtimeDir ?: File(desktopDataRoot())
        val chosen = choose(System.getenv("FONTCONFIG_FILE"), resourcesDir()?.let { File(it, SHIPPED) }) { writeWrapper(it, dir) }
        val (file, why) = chosen.getOrElse { e -> warn("the wrapper could not be written in $dir: $e"); return }
        val rc = runCatching { Native.load("c", LibcEnv::class.java).setenv("FONTCONFIG_FILE", file.path, 1) }
        if (rc.getOrNull() != 0) { warn("setenv(FONTCONFIG_FILE) failed: ${rc.exceptionOrNull() ?: "returned ${rc.getOrNull()}"}"); return }
        inUse = file
        how = why + if (file.parentFile == runtimeDir) " in XDG_RUNTIME_DIR" else if (file.parentFile?.path == dir.path) " in the data root" else ""
    }

    /**
     * [pb] with FONTCONFIG_FILE as it was at launch (removed when it was unset), so a program the app starts doesn't
     * inherit this process's setenv. Set explicitly: a ProcessBuilder whose environment is never touched passes on the
     * process's current environment, setenv included.
     */
    fun childEnvironment(pb: ProcessBuilder, launch: String? = if (readAtLaunch) atLaunch else System.getenv("FONTCONFIG_FILE")) =
        pb.apply { environment().apply { if (launch == null) remove("FONTCONFIG_FILE") else put("FONTCONFIG_FILE", launch) } }

    /** The wrapper of [include] as [WRAPPER] in [dir], replaced whole (written beside it, then moved over it). Throws. */
    internal fun writeWrapper(include: String, dir: File): File {
        dir.mkdirs()
        val tmp = Files.createTempFile(dir.toPath(), "troubastage-fonts", ".tmp")
        try {
            Files.writeString(tmp, wrapper(include))
            return Files.move(tmp, File(dir, WRAPPER).toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE).toFile()
        } finally {
            Files.deleteIfExists(tmp)
        }
    }

    private fun warn(why: String) {
        how = "not applied: $why"
        System.err.println("$WARNING ($why); text that only a bitmap font covers may crash the app.")
    }

    private fun xmlEscape(s: String) = s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")

    private interface LibcEnv : Library {
        fun setenv(name: String, value: String, overwrite: Int): Int
    }
}
