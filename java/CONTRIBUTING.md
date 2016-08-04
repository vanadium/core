# Contributing to Vanadium

Vanadium is an open source project.

It is the work of many contributors. We appreciate your help!

## Filing issues

We use a single GitHub repository for [tracking all
issues](https://github.com/vanadium/issues/issues) across all Vanadium
repositories.

## Contributing code

Please read the [contribution
guidelines](https://vanadium.github.io/community/contributing.html) before
sending patches.

**We do not accept GitHub pull requests.** (We use
[Gerrit](https://www.gerritcodereview.com/) instead for code reviews.)

Unless otherwise noted, the Vanadium source files are distributed under the
BSD-style license found in the LICENSE file.

## Testing changes

Typical users of the Vanadium for Java libraries will use a dependency manager
such as Maven or Gradle to bring Vanadium into their projects.

For example, in an Android project, the user may do something like:

```groovy
// MyAndroidProject/app/build.gradle

apply plugin: 'com.android.application'

// ...

repositories {
    jCenter()
}

dependencies {
    compile 'io.v:vanadium-android:1.6'
}
```

Using Gradle to build the application will cause a binary JAR version of
Vanadium for Android to be downloaded from a Maven repository on the
Internet (in this case, the JCenter repository).

While this is very convenient for the end user, it is not so convenient to the
Vanadium contributor who wishes to test changes without making a full
release.

To be able to incorporate local changes to underlying libraries, we
make use of the fact that Maven can be configured to use a repository
on the local filesystem.
[Gradle configuration](http://developer.android.com/tools/building/configuring-gradle.html)
gives us convenient access to this repository under the `mavenLocal`
moniker.

The following instructions do that, and assume you work from the Java
root directory, e.g.

```
cd ~/vanadium/release/java
```

#### Modify Go library construction

Edit

```
lib/build.gradle
```

Set `releaseVersion` to a new unused value, e.g. `9.9`.


#### Modify android library construction

Edit

```
android-lib/build.gradle
```

 1. Set `releaseVersion` to the same value used above.
 2. Add `mavenLocal()` before `jCenter()` in the `buildscript` stanzas.


#### Modify project construction

Edit

```
${yourProject}/build.gradle   # project level
```

Add `mavenLocal()` _before_ `jCenter()` in the
`allprojects/repositories` stanza.


#### Modify module construction

Edit

```
${yourProject}/app/build.gradle   # module level
```

Change the `dependencies` stanza to specify your new version number, e.g.

```
compile 'io.v:vanadium-android:9.9'
```

#### Purge caches

```
./gradlew :lib:clean :android-lib:clean
```

#### Deploy locally

```
./gradlew :lib:publishToMavenLocal :android-lib:publishToMavenLocal
```

You can now build your project against local
upstrean changes, e.g.

```
(cd ${yourProject}; ./gradlew assembleDebug)
```
