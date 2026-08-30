pluginManagement {
    repositories {
        google()
        maven { url = uri("https://repo1.maven.org/maven2") }
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        maven { url = uri("https://repo1.maven.org/maven2") }
    }
}

rootProject.name = "Ferry"
include(":app")
