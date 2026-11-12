plugins {
    id("com.android.application") version "9.4.1"
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.21"
}

// Where the build writes: the repository's bin/, beside the binaries, so that
// `task clean` takes it with them and the tree's own directories stay the
// project's structure - a build directory is full of plurals.
layout.buildDirectory = file("../../../bin/mobile")

// The release this app belongs to, as the release workflow names it: v0.3.95.
// The installation reports its own the same way, and the app compares the two.
val release: String = providers.environmentVariable("VERSION").getOrElse("dev")

// versionCodeOf is the number Android tells a newer app from an older one by.
// Each part of a release counts to 99 and then hands on (build/nextversion), so
// a hundred per part keeps every later release larger. A build that is not a
// release is 1.
fun versionCodeOf(version: String): Int {
    val (major, minor, patch) = Regex("""^v(\d+)\.(\d+)\.(\d+)$""").find(version)?.destructured
        ?: return 1

    return major.toInt() * 10_000 + minor.toInt() * 100 + patch.toInt()
}

// The key a release is signed with, handed over by the release workflow from
// its secrets. Android installs an update only over an app signed with the same
// key, so it is one key for every release, kept where the workflow can reach it
// and nowhere in the repository. A build without it - every pull request's -
// makes an unsigned release APK, which proves everything but the signature.
val keystore: String? = providers.environmentVariable("GTS_ANDROID_KEYSTORE").orNull

android {
    namespace = "io.github.dennisdko.goticketsystem"

    // The androidx libraries are compiled against 37 and ask the app to be too.
    // It targets 36 on purpose: an app targeting 37 needs a runtime permission to
    // reach the local network - where an installation without a public domain
    // is - and has Certificate Transparency enforced, which its operator's own
    // CA has no log for. Both are to be measured against such an installation
    // before the target moves.
    compileSdk = 37

    defaultConfig {
        applicationId = "io.github.dennisdko.goticketsystem"
        minSdk = 26
        targetSdk = 36
        versionCode = versionCodeOf(release)
        versionName = release
    }

    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = file(keystore)
                storePassword = providers.environmentVariable("GTS_ANDROID_KEYSTORE_PASSWORD").get()
                keyAlias = providers.environmentVariable("GTS_ANDROID_KEY_ALIAS").get()
                keyPassword = providers.environmentVariable("GTS_ANDROID_KEY_PASSWORD").get()
            }
        }
    }

    buildTypes {
        getByName("release") {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            signingConfig = signingConfigs.findByName("release")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    lint {
        abortOnError = true
        checkReleaseBuilds = true

        // The target is 36 for the reason above, and every version here is
        // pinned and moved on a deliberate bump, as golangci-lint's is - so lint
        // saying a newer one exists says nothing anybody would act on.
        disable += setOf("OldTargetApi", "AndroidGradlePluginVersion", "GradleDependency", "NewerVersionAvailable")
    }
}

// The application's dictionaries, the very files the server reads, copied into
// the app as it is built rather than kept as a copy here - so the app speaks
// every language the installation does from the first screen on, before there
// is an installation to ask, and a new language is still only its file.
androidComponents {
    onVariants { variant ->
        variant.sources.assets?.addStaticSourceDirectory("../../support/i18n/locale")
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2026.09.00"))
    implementation("androidx.compose.material3:material3")
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.core:core-ktx:1.18.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.11.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.11.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.11.0")

    // The scanner for a token's code: CameraX for the camera and ZXing for the
    // code, so that it scans on a phone without Google Play services as well -
    // which Google's own code scanner would need.
    implementation("androidx.camera:camera-core:1.6.2")
    implementation("androidx.camera:camera-camera2:1.6.2")
    implementation("androidx.camera:camera-lifecycle:1.6.2")
    implementation("androidx.camera:camera-view:1.6.2")
    implementation("com.google.zxing:core:3.5.4")

    // The reminders, asked for every quarter of an hour and at the moment the
    // stopwatch is due, whether the app is open or not - without a push service,
    // and so without Google Play services.
    implementation("androidx.work:work-runtime-ktx:2.12.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.json:json:20260814")
}
