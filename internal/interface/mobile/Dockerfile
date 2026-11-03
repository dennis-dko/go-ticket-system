# The toolchain the Android app is built with, the same on a laptop and in CI: a
# JDK, one pinned Gradle and the parts of the Android SDK the build asks for.
#
# Built without a context - `docker build - < Dockerfile` - because nothing of
# the repository belongs in it. The sources are mounted when it runs, so the
# image changes only when a version below does, and a laptop and CI that built
# it from this file build the app with the same tools.
FROM eclipse-temurin:17.0.20_8-jdk-noble

# Gradle itself rather than a wrapper. A wrapper is a jar committed to the
# repository that downloads and runs a build tool; naming the version and its
# checksum here does the same without a binary nobody reads. The checksum is
# the one Gradle publishes beside the archive.
ARG GRADLE_VERSION=9.8.1
ARG GRADLE_SHA256=dce76f55f8e251a3a1f130eb120f30b3d271de2b76c9b0729d316b5a1b6dc01f

# The SDK's command-line tools 23.0, checked against the SHA-1 Google lists for
# this archive in its own repository index, repository2-3.xml - the only
# checksum it publishes for it.
ARG TOOLS_BUILD=16111833
ARG TOOLS_SHA1=e025545c62a8e64c7559119566a569fb1dec5f60

ENV ANDROID_HOME=/opt/android-sdk
ENV PATH=/opt/gradle/bin:${ANDROID_HOME}/cmdline-tools/latest/bin:${PATH}

RUN apt-get update \
 && apt-get install -y --no-install-recommends curl unzip \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSLo /tmp/gradle.zip "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" \
 && echo "${GRADLE_SHA256}  /tmp/gradle.zip" | sha256sum -c - \
 && unzip -q /tmp/gradle.zip -d /opt \
 && mv "/opt/gradle-${GRADLE_VERSION}" /opt/gradle \
 && rm /tmp/gradle.zip

RUN curl -fsSLo /tmp/tools.zip "https://dl.google.com/android/repository/commandlinetools-linux-${TOOLS_BUILD}_latest.zip" \
 && echo "${TOOLS_SHA1}  /tmp/tools.zip" | sha1sum -c - \
 && mkdir -p "${ANDROID_HOME}/cmdline-tools" \
 && unzip -q /tmp/tools.zip -d /tmp/tools \
 && mv /tmp/tools/cmdline-tools "${ANDROID_HOME}/cmdline-tools/latest" \
 && rm -rf /tmp/tools /tmp/tools.zip

# The platform the app compiles against, the build tools Android Gradle plugin
# 9.4 asks for and the platform tools it installs when they are missing -
# installed here so a build downloads only its Maven dependencies. sdkmanager
# checks each package against the repository's own checksum. The SDK cannot be
# used until its licences are accepted, which is what building the app with it
# means.
RUN yes | sdkmanager --licenses > /dev/null \
 && sdkmanager --install "platforms;android-37.0" "build-tools;36.0.0" "platform-tools" > /dev/null
