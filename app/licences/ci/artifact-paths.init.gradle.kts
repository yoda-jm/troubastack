// A101 CI only — the artifact lookup the notices check uses (not the generator's code): prints "g:a:v<TAB>path" for each
// external component of the shipped runtime classpath (:androidApp's debugRuntimeClasspath — its AARs and jars;
// :desktopApp's runtimeClasspath — its jars) to -Da101.out. Run with --no-configuration-cache.
allprojects {
    val cfgName = when (path) { ":androidApp" -> "debugRuntimeClasspath"; ":desktopApp" -> "runtimeClasspath"; else -> null }
    // The project's path, read here: inside the task's block below, `path` is the TASK's (":androidApp:a101ArtifactPaths"),
    // which once made Android ask for jars only — AGP's transformed classes.jar, without the AAR's jni/ natives.
    val types = if (path == ":androidApp") listOf("aar", "jar") else listOf("jar")
    if (cfgName != null) afterEvaluate {
        tasks.register("a101ArtifactPaths") {
            doLast {
                val type = org.gradle.api.attributes.Attribute.of("artifactType", String::class.java)
                val cfg = project.configurations.getByName(cfgName)
                val seen = LinkedHashMap<String, java.io.File>()
                for (t in types) {
                    cfg.incoming.artifactView {
                        attributes { attribute(type, t) }
                        componentFilter { it is org.gradle.api.artifacts.component.ModuleComponentIdentifier }
                    }.artifacts.artifacts.forEach { r ->
                        val c = r.id.componentIdentifier as org.gradle.api.artifacts.component.ModuleComponentIdentifier
                        seen.putIfAbsent("${c.group}:${c.module}:${c.version}", r.file)
                    }
                }
                java.io.File(System.getProperty("a101.out")).writeText(seen.entries.joinToString("") { "${it.key}\t${it.value}\n" })
                println("a101ArtifactPaths ($path, ${types.joinToString("+")}): ${seen.size} components, ${seen.values.count { it.name.endsWith(".aar") }} AARs")
            }
        }
    }
}
