// A101 — build logic both apps share: OpenSourceNotices (the Licences screen's list, generated from what each build
// resolves). Plain Java on Gradle's API only: no Kotlin Gradle plugin here, which would clash with the KMP plugin the
// modules apply.
plugins {
    java
}

dependencies {
    implementation(gradleApi())
}
