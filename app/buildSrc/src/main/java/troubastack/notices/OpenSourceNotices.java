package troubastack.notices;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Enumeration;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.zip.ZipEntry;
import java.util.zip.ZipFile;
import java.util.zip.ZipInputStream;
import org.gradle.api.DefaultTask;
import org.gradle.api.GradleException;
import org.gradle.api.artifacts.component.ModuleComponentIdentifier;
import org.gradle.api.artifacts.result.ResolvedArtifactResult;
import org.gradle.api.file.ConfigurableFileCollection;
import org.gradle.api.file.DirectoryProperty;
import org.gradle.api.provider.Property;
import org.gradle.api.provider.SetProperty;
import org.gradle.api.tasks.Input;
import org.gradle.api.tasks.InputDirectory;
import org.gradle.api.tasks.InputFiles;
import org.gradle.api.tasks.Internal;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.PathSensitive;
import org.gradle.api.tasks.PathSensitivity;
import org.gradle.api.tasks.TaskAction;

/**
 * A101 — writes {@code <relativeDir>/index.txt} and {@code <relativeDir>/texts/*}: the open-source notices of every
 * external component on the build's shipped runtime classpath, which the Licences screen lists. Per component: its
 * POM licences (parents followed) mapped to committed SPDX texts (an unmapped one fails the build), its elections and
 * extra licences, and every NOTICE / licence / copyright file inside its artifact, verbatim. Plus the native code it
 * carries ({@code native/manifest.txt}, version-pinned). Each text is stored once, named by its content.
 *
 * <p>index.txt, one record per line, tab-separated:
 * <pre>
 * entry      name
 * component  group:artifact:version  spdx-ids(,)  licence summary  verbatim-text-ids(,)
 * carrier    group:artifact:version                  (a native entry: the component that carries it)
 * doc        title  text-id
 * delegated  group:artifact:version  spdx-ids  where its texts are   (listed, texts in an entry built elsewhere)
 * </pre>
 * Sabotages ({@code -Pa101Sabotage}, a CI measurement run only): {@code licencedropone} leaves one component out;
 * {@code licencenonotice} leaves OkHttp's NOTICE out.
 */
public abstract class OpenSourceNotices extends DefaultTask {
    static final String DROP_ONE = "com.squareup.okio:okio-jvm";
    static final String NOTICE_DROPPED = "com.squareup.okhttp3:okhttp";
    private static final Pattern VERBATIM = Pattern.compile("(?i)(^|/)(notice|licen[cs]e|copying|copyright)[^/]*$");
    private static final Map<String, String> SPDX_NAMES = Map.of(
            "Apache-2.0", "Apache License 2.0",
            "MIT", "MIT License",
            "BSD-3-Clause", "BSD 3-Clause License",
            "MPL-2.0", "Mozilla Public License 2.0",
            "EPL-2.0", "Eclipse Public License 2.0");

    @Internal public abstract SetProperty<ResolvedArtifactResult> getArtifacts();
    @InputFiles @PathSensitive(PathSensitivity.NAME_ONLY) public abstract ConfigurableFileCollection getArtifactFiles();
    @Internal public abstract SetProperty<ResolvedArtifactResult> getPoms();
    @InputFiles @PathSensitive(PathSensitivity.NAME_ONLY) public abstract ConfigurableFileCollection getPomFiles();
    @InputDirectory @PathSensitive(PathSensitivity.RELATIVE) public abstract DirectoryProperty getLicencesDir();
    @Input public abstract Property<String> getPlatform();
    @Input public abstract Property<String> getSabotage();
    @Input public abstract Property<String> getRelativeDir();
    @OutputDirectory public abstract DirectoryProperty getOutputDir();

    private record Component(String gav, String group, String artifact, String version, File file) {
        String ga() { return group + ":" + artifact; }
    }

    private static final class Entry {
        final String name;
        final List<String> lines = new ArrayList<>();
        final Set<String> docIds = new LinkedHashSet<>();
        final List<String> docLines = new ArrayList<>();
        final Set<String> licenceIds = new LinkedHashSet<>();
        Entry(String name) { this.name = name; }
        void doc(String title, String id) { if (docIds.add(id)) docLines.add("doc\t" + title + "\t" + id); }
    }

    @TaskAction
    public void generate() throws IOException {
        long t0 = System.nanoTime();
        File licences = getLicencesDir().get().getAsFile();
        String platform = getPlatform().get();
        String sabotage = getSabotage().get();
        List<String[]> licenceMap = table(new File(licences, "licence-map.txt"), 2);
        Map<String, String[]> elections = keyed(table(new File(licences, "elections.txt"), 3));
        Map<String, String[]> extras = keyed(table(new File(licences, "extra-licences.txt"), 3));
        Map<String, String[]> delegated = keyed(table(new File(licences, "delegated.txt"), 2));
        List<String[]> families = table(new File(licences, "families.txt"), 2);
        List<String[]> natives = table(new File(licences, "native/manifest.txt"), 6);

        Map<String, String> poms = new LinkedHashMap<>();
        for (ResolvedArtifactResult r : getPoms().get()) {
            if (r.getId().getComponentIdentifier() instanceof ModuleComponentIdentifier m) {
                poms.put(m.getGroup() + ":" + m.getModule() + ":" + m.getVersion(), Files.readString(r.getFile().toPath(), StandardCharsets.UTF_8));
            }
        }
        // One artifact per component: an AAR wins over a jar the same component also offers (Android).
        Map<String, Component> components = new TreeMap<>();
        for (ResolvedArtifactResult r : getArtifacts().get()) {
            if (!(r.getId().getComponentIdentifier() instanceof ModuleComponentIdentifier m)) continue;
            String gav = m.getGroup() + ":" + m.getModule() + ":" + m.getVersion();
            Component c = new Component(gav, m.getGroup(), m.getModule(), m.getVersion(), r.getFile());
            Component had = components.get(gav);
            if (had == null || (r.getFile().getName().endsWith(".aar") && !had.file().getName().endsWith(".aar"))) components.put(gav, c);
        }

        File out = getOutputDir().get().getAsFile();
        deleteRecursively(out);
        File dir = new File(out, getRelativeDir().get());
        File texts = new File(dir, "texts");
        if (!texts.mkdirs()) throw new GradleException("cannot create " + texts);

        List<String> errors = new ArrayList<>();
        Map<String, Entry> entries = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
        List<String> delegatedLines = new ArrayList<>();
        int verbatimCount = 0;
        for (Component c : components.values()) {
            if ("licencedropone".equals(sabotage) && c.ga().equals(DROP_ONE)) continue;
            List<String> pomLicences = pomLicences(poms, c.gav(), 0, errors);
            Set<String> ids = new LinkedHashSet<>();
            for (String l : pomLicences) {
                String id = mapLicence(licenceMap, l);
                if (id == null) errors.add(c.gav() + ": no SPDX id for its POM licence \"" + l + "\" — add a row to licence-map.txt");
                else ids.add(id);
            }
            String summary = String.join(", ", ids);
            String[] election = elections.get(c.ga());
            if (election != null) {
                ids.clear();
                ids.add(election[1]);
                summary = election[2];
            }
            String[] extra = extras.get(c.ga());
            if (extra != null) {
                String checked = extra[0];
                if (!checked.equals(c.gav())) errors.add(c.gav() + ": extra-licences.txt was checked for " + checked + " — check this version's shipped files and update the row");
                ids.add(extra[1]);
                summary += "; " + extra[2] + " " + extra[1];
            }
            if (delegated.containsKey(c.ga())) {
                delegatedLines.add("delegated\t" + c.gav() + "\t" + String.join(",", ids) + "\t" + delegated.get(c.ga())[1]);
                continue;
            }
            String family = family(families, c, poms.get(c.gav()));
            Entry e = entries.computeIfAbsent(family, Entry::new);
            List<String> verbatimIds = new ArrayList<>();
            for (Map.Entry<String, byte[]> v : verbatim(c.file()).entrySet()) {
                if ("licencenonotice".equals(sabotage) && c.ga().equals(NOTICE_DROPPED)
                        && v.getKey().substring(v.getKey().lastIndexOf('/') + 1).toLowerCase(Locale.ROOT).startsWith("notice")) continue;
                String id = store(texts, v.getValue());
                verbatimIds.add(id);
                e.doc(c.artifact() + ": " + v.getKey(), id);
                verbatimCount++;
            }
            e.lines.add("component\t" + c.gav() + "\t" + String.join(",", ids) + "\t" + summary + "\t" + String.join(",", verbatimIds));
            e.licenceIds.addAll(ids);
            for (String[] n : natives) {
                if (!matches(n[0], c.ga()) || !appliesTo(n[3], platform)) continue;
                if (!n[1].equals(c.version())) {
                    errors.add(c.gav() + ": native/manifest.txt was checked for version " + n[1] + " — check what this version's native library bundles and update the manifest");
                    continue;
                }
                Entry ne = entries.computeIfAbsent(n[2], Entry::new);
                String carrier = "carrier\t" + c.gav();
                if (!ne.lines.contains(carrier)) ne.lines.add(carrier);
                ne.doc(n[5], store(texts, Files.readAllBytes(new File(licences, "native/" + n[4]).toPath())));
            }
        }
        // Each entry's licence texts, once, after its verbatim files.
        for (Entry e : entries.values()) {
            for (String id : e.licenceIds) {
                File spdx = new File(licences, "spdx/" + id + ".txt");
                if (!spdx.isFile()) { errors.add("no committed text spdx/" + id + ".txt for entry " + e.name); continue; }
                e.doc(SPDX_NAMES.getOrDefault(id, id), store(texts, Files.readAllBytes(spdx.toPath())));
            }
        }
        if (!errors.isEmpty()) throw new GradleException("A101 open-source notices:\n  " + String.join("\n  ", errors));

        StringBuilder index = new StringBuilder("# A101 — open-source notices, generated by OpenSourceNotices from this build's resolved runtime classpath (")
                .append(platform).append(")\n");
        for (Entry e : entries.values()) {
            index.append("entry\t").append(e.name).append('\n');
            for (String l : e.lines) index.append(l).append('\n');
            for (String l : e.docLines) index.append(l).append('\n');
        }
        for (String l : delegatedLines) index.append(l).append('\n');
        Files.writeString(new File(dir, "index.txt").toPath(), index.toString(), StandardCharsets.UTF_8);
        File[] stored = texts.listFiles();
        long bytes = 0;
        for (File f : stored == null ? new File[0] : stored) bytes += f.length();
        getLogger().lifecycle("openSourceNotices ({}): {} components, {} entries, {} verbatim files, {} texts ({} KB) in {} ms",
                platform, components.size(), entries.size(), verbatimCount, stored == null ? 0 : stored.length, bytes / 1024,
                (System.nanoTime() - t0) / 1_000_000);
    }

    /** A POM's licences ("name &lt;url&gt;"), from its parents when it declares none. */
    private static List<String> pomLicences(Map<String, String> poms, String gav, int depth, List<String> errors) {
        String xml = poms.get(gav);
        if (xml == null) { errors.add(gav + ": its POM was not resolved"); return List.of(); }
        List<String> out = new ArrayList<>();
        Matcher m = Pattern.compile("(?s)<license>(.*?)</license>").matcher(xml);
        while (m.find()) {
            String name = tag(m.group(1), "name"), url = tag(m.group(1), "url");
            out.add((name == null ? "" : name) + " <" + (url == null ? "" : url) + ">");
        }
        if (!out.isEmpty()) return out;
        String parent = NoticesSetup.parentOf(xml);
        if (parent == null || depth > 6) { errors.add(gav + ": its POM names no licence, nor does any parent"); return List.of(); }
        return pomLicences(poms, parent, depth + 1, errors);
    }

    static String tag(String xml, String tag) {
        Matcher m = Pattern.compile("(?s)<" + tag + ">(.*?)</" + tag + ">").matcher(xml);
        return m.find() ? m.group(1).trim() : null;
    }

    private static String mapLicence(List<String[]> map, String licence) {
        String l = licence.toLowerCase(Locale.ROOT);
        for (String[] row : map) if (l.contains(row[0].toLowerCase(Locale.ROOT))) return row[1];
        return null;
    }

    private static String family(List<String[]> families, Component c, String pom) {
        for (String[] f : families) if (c.ga().startsWith(f[0])) return f[1];
        String name = pom == null ? null : tag(pom.replaceAll("(?s)<parent>.*?</parent>", "")
                .replaceAll("(?s)<(licenses|developers|organization|scm|dependencies)>.*", ""), "name");
        return name == null || name.isBlank() ? c.artifact() : name;
    }

    private static boolean matches(String pattern, String ga) {
        return pattern.endsWith("*") ? ga.startsWith(pattern.substring(0, pattern.length() - 1)) : ga.equals(pattern);
    }

    private static boolean appliesTo(String oses, String platform) {
        for (String os : oses.split(",")) if (os.trim().equals(platform)) return true;
        return false;
    }

    /** Every NOTICE / licence / copyright file inside the artifact (and an AAR's classes.jar), verbatim, by path. */
    private static Map<String, byte[]> verbatim(File artifact) throws IOException {
        Map<String, byte[]> out = new TreeMap<>();
        try (ZipFile zip = new ZipFile(artifact)) {
            Enumeration<? extends ZipEntry> all = zip.entries();
            while (all.hasMoreElements()) {
                ZipEntry e = all.nextElement();
                if (e.isDirectory()) continue;
                if (e.getName().endsWith(".class")) continue;
                if (VERBATIM.matcher(e.getName()).find()) {
                    try (InputStream in = zip.getInputStream(e)) { out.put(e.getName(), in.readAllBytes()); }
                } else if (e.getName().equals("classes.jar")) {
                    try (ZipInputStream nested = new ZipInputStream(zip.getInputStream(e))) {
                        for (ZipEntry n; (n = nested.getNextEntry()) != null; ) {
                            if (!n.isDirectory() && !n.getName().endsWith(".class") && VERBATIM.matcher(n.getName()).find()) {
                                out.put("classes.jar!" + n.getName(), nested.readAllBytes());
                            }
                        }
                    }
                }
            }
        }
        return out;
    }

    /** Store a text once, under the hex of its SHA-256's first 8 bytes; returns that id. */
    private static String store(File texts, byte[] content) throws IOException {
        String id = sha(content);
        File f = new File(texts, id + ".txt");
        if (!f.exists()) Files.write(f.toPath(), content);
        return id;
    }

    static String sha(byte[] content) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(content)).substring(0, 16);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException(e);
        }
    }

    /** Rows of a tab-separated table, skipping blank lines and # comments; each row must have [columns] fields. */
    private static List<String[]> table(File file, int columns) throws IOException {
        List<String[]> rows = new ArrayList<>();
        for (String line : Files.readAllLines(file.toPath(), StandardCharsets.UTF_8)) {
            if (line.isBlank() || line.startsWith("#")) continue;
            String[] f = line.split("\t");
            if (f.length < columns) throw new GradleException(file + ": a row needs " + columns + " tab-separated fields: " + line);
            rows.add(f);
        }
        return rows;
    }

    /** Rows keyed by their first field's group:artifact (a group:artifact:version key is shortened to group:artifact). */
    private static Map<String, String[]> keyed(List<String[]> rows) {
        Map<String, String[]> out = new LinkedHashMap<>();
        for (String[] r : rows) {
            String[] p = r[0].split(":");
            out.put(p[0] + ":" + p[1], r);
        }
        return out;
    }

    private static void deleteRecursively(File f) throws IOException {
        if (!f.exists()) return;
        File[] kids = f.listFiles();
        if (kids != null) for (File k : kids) deleteRecursively(k);
        Files.delete(f.toPath());
    }
}
