package troubastack.notices;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.gradle.api.Project;
import org.gradle.api.artifacts.ArtifactCollection;
import org.gradle.api.artifacts.Configuration;
import org.gradle.api.artifacts.Dependency;
import org.gradle.api.artifacts.ExternalModuleDependency;
import org.gradle.api.artifacts.component.ModuleComponentIdentifier;
import org.gradle.api.artifacts.dsl.DependencyFactory;
import org.gradle.api.artifacts.result.DependencyResult;
import org.gradle.api.artifacts.result.ResolvedArtifactResult;
import org.gradle.api.artifacts.result.ResolvedComponentResult;
import org.gradle.api.artifacts.result.ResolvedDependencyResult;
import org.gradle.api.attributes.Attribute;
import org.gradle.api.tasks.TaskProvider;

/**
 * A101 — wires an {@link OpenSourceNotices} task to a module's shipped runtime classpath. Nothing resolves while the
 * build is configured: the artifacts are task inputs, and the POMs (with their parents, for the licences a parent
 * declares) come from detached configurations whose dependencies are derived lazily from the classpath's resolution
 * result and then from the previous level's POM files. Four levels cover the deepest parent chain the apps have
 * (slf4j-api → slf4j-bom → slf4j-parent).
 */
public final class NoticesSetup {
    private static final Attribute<String> ARTIFACT_TYPE = Attribute.of("artifactType", String.class);
    private static final int POM_LEVELS = 4;
    private static final Pattern PARENT = Pattern.compile("(?s)<parent>(.*?)</parent>");

    private NoticesSetup() {}

    /**
     * @param artifactTypes the artifact types the build packages: {"aar", "jar"} on Android, {"jar"} on the desktop
     * @param platform android / linux / windows / macos — selects the native texts that apply
     * @param relativeDir where index.txt and texts/ go inside the output directory
     */
    public static TaskProvider<OpenSourceNotices> register(
            Project project, String taskName, Configuration runtime, String[] artifactTypes, String platform,
            String relativeDir, File licencesDir) {
        List<ArtifactCollection> views = new ArrayList<>();
        for (String type : artifactTypes) {
            views.add(runtime.getIncoming().artifactView(v -> {
                v.attributes(a -> a.attribute(ARTIFACT_TYPE, type));
                v.componentFilter(id -> id instanceof ModuleComponentIdentifier);
            }).getArtifacts());
        }
        DependencyFactory factory = project.getDependencyFactory();
        List<Configuration> pomLevels = new ArrayList<>();
        Configuration level0 = project.getConfigurations().detachedConfiguration();
        level0.setTransitive(false);
        level0.getDependencies().addAllLater(
                runtime.getIncoming().getResolutionResult().getRootComponent().map(root -> pomDependencies(factory, modules(root))));
        pomLevels.add(level0);
        for (int i = 1; i < POM_LEVELS; i++) {
            Configuration previous = pomLevels.get(i - 1);
            Configuration level = project.getConfigurations().detachedConfiguration();
            level.setTransitive(false);
            level.getDependencies().addAllLater(
                    previous.getIncoming().getArtifacts().getResolvedArtifacts().map(poms -> pomDependencies(factory, parents(poms))));
            pomLevels.add(level);
        }
        String sabotage = project.getProviders().gradleProperty("a101Sabotage").getOrElse("");
        return project.getTasks().register(taskName, OpenSourceNotices.class, t -> {
            t.setGroup("build");
            t.setDescription("A101: the open-source notices of every component this build ships, for the Licences screen");
            for (ArtifactCollection view : views) {
                t.getArtifacts().addAll(view.getResolvedArtifacts());
                t.getArtifactFiles().from(view.getArtifactFiles());
            }
            for (Configuration level : pomLevels) {
                t.getPoms().addAll(level.getIncoming().getArtifacts().getResolvedArtifacts());
                t.getPomFiles().from(level.getIncoming().getArtifacts().getArtifactFiles());
            }
            t.getLicencesDir().set(licencesDir);
            t.getPlatform().set(platform);
            t.getSabotage().set(sabotage);
            t.getRelativeDir().set(relativeDir);
            t.getOutputDir().set(project.getLayout().getBuildDirectory().dir("notices/" + taskName));
        });
    }

    /** Every external module in the resolved graph (BOMs, platforms and KMP root modules too: their POMs are small). */
    static Set<String> modules(ResolvedComponentResult root) {
        Set<String> out = new LinkedHashSet<>();
        Set<ResolvedComponentResult> seen = new HashSet<>();
        List<ResolvedComponentResult> todo = new ArrayList<>(List.of(root));
        while (!todo.isEmpty()) {
            ResolvedComponentResult c = todo.remove(todo.size() - 1);
            if (!seen.add(c)) continue;
            if (c.getId() instanceof ModuleComponentIdentifier m) out.add(m.getGroup() + ":" + m.getModule() + ":" + m.getVersion());
            for (DependencyResult d : c.getDependencies()) {
                if (d instanceof ResolvedDependencyResult r) todo.add(r.getSelected());
            }
        }
        return out;
    }

    /** The parents the given POMs name. */
    static Set<String> parents(Set<ResolvedArtifactResult> poms) {
        Set<String> out = new LinkedHashSet<>();
        for (ResolvedArtifactResult pom : poms) {
            try {
                String xml = Files.readString(pom.getFile().toPath(), StandardCharsets.UTF_8);
                String parent = parentOf(xml);
                if (parent != null) out.add(parent);
            } catch (IOException e) {
                throw new IllegalStateException("cannot read " + pom.getFile(), e);
            }
        }
        return out;
    }

    /** "g:a:v" of a POM's &lt;parent&gt;, or null. */
    static String parentOf(String xml) {
        Matcher m = PARENT.matcher(xml);
        if (!m.find()) return null;
        String p = m.group(1);
        String g = OpenSourceNotices.tag(p, "groupId"), a = OpenSourceNotices.tag(p, "artifactId"), v = OpenSourceNotices.tag(p, "version");
        return g == null || a == null || v == null ? null : g + ":" + a + ":" + v;
    }

    private static List<Dependency> pomDependencies(DependencyFactory factory, Set<String> gavs) {
        List<Dependency> out = new ArrayList<>();
        for (String gav : gavs) {
            String[] p = gav.split(":");
            ExternalModuleDependency d = factory.create(p[0], p[1], p[2]);
            d.setTransitive(false);
            d.artifact(a -> {
                a.setName(p[1]);
                a.setType("pom");
                a.setExtension("pom");
            });
            out.add(d);
        }
        return out;
    }
}
