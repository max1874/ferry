buildscript {
    dependencies {
        // AGP 9 compiles Kotlin itself (no org.jetbrains.kotlin.android plugin); this pins the Kotlin
        // compiler to the same version as the Compose compiler plugin below.
        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:2.4.21")
    }
}

plugins {
    id("com.android.application") version "9.4.1" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.21" apply false
}
