import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

val releaseSigningFile = rootProject.file("signing.properties")
val releaseSigning = Properties()

if (releaseSigningFile.isFile) {
    releaseSigningFile.inputStream().use(releaseSigning::load)
}

fun requiredSigningProperty(name: String): String =
    releaseSigning.getProperty(name)?.takeIf { it.isNotBlank() }
        ?: throw GradleException("android/signing.properties is missing required property: $name")

android {
    namespace = "com.max1874.ferry"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.max1874.ferry"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "1.0.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildFeatures { compose = true }

    signingConfigs {
        if (releaseSigningFile.isFile) {
            create("release") {
                val configuredStore = rootProject.file(requiredSigningProperty("storeFile"))
                if (!configuredStore.isFile) {
                    throw GradleException("Android release keystore does not exist: $configuredStore")
                }
                storeFile = configuredStore
                storePassword = requiredSigningProperty("storePassword")
                keyAlias = requiredSigningProperty("keyAlias")
                keyPassword = requiredSigningProperty("keyPassword")
            }
        }
    }

    buildTypes {
        getByName("release") {
            signingConfig = signingConfigs.findByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }

    testOptions { unitTests.isReturnDefaultValues = true }

    packaging {
        resources.excludes += setOf("/META-INF/{AL2.0,LGPL2.1}")
    }
}

gradle.taskGraph.whenReady {
    val packagesRelease = allTasks.any {
        it.path == ":app:packageRelease" || it.path == ":app:bundleRelease"
    }
    if (packagesRelease && !releaseSigningFile.isFile) {
        throw GradleException(
            "Release signing is not configured. Copy signing.properties.example to signing.properties and fill it locally."
        )
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2025.05.01")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    implementation("androidx.core:core-ktx:1.16.0")
    implementation("androidx.activity:activity-compose:1.10.1")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.compose.ui:ui-tooling-preview")
    debugImplementation("androidx.compose.ui:ui-tooling")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.json:json:20240303")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
}
