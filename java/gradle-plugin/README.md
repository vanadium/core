Vanadium Gradle Plugin
======================

This plugin provides users of [Gradle](https://gradle.org/) with a convenient
way to generate Java
[VDL](https://vanadium.github.io/designdocs/vdl-spec.html)
wrappers and to use those wrappers in a Java or Android project. It automates
several tasks that projects using VDL commonly have to perform.

# Usage

To use the plugin, include the following configuration in your Gradle's
`buildscript` block:

```groovy
buildscript {
    repositories {
        jcenter()
    }

    dependencies {
        classpath 'io.v:gradle-plugin:0.5'
    }
}
```

Then apply the plugin:

```groovy
apply plugin: 'io.v.vdl'
```

## Configuration

The plugin allows you to introduce a `vdl` section into your `build.gradle` to
specify configuration options. For example:

```groovy
vdl {
    inputPaths += 'src/main/java'
}
```

Here is a full list of the configuration options:

| Option | Type | Description | Default value |
|-----------------------|----------------|-----------------------------------------------------------------------------------|----------------------------------|
| `inputPaths` | `List<String>` | The list of paths that will be searched for VDL files | `[]` |
| `outputPath` | `String` | The path where generated Java sources will be placed | `"generated-src/vdl"` |
| `packageTranslations` | `List<String>` | A list of package translations to perform (see the corresponding [VDL option]) | `["v.io->io/v"]` |
| `transitiveVdlDir` | `String` | The path where VDL files in this project's transitive dependencies will be placed (see [below](#libraries-and-dependencies)) | `"generated-src/transitive-vdl"` |
| `vdlRootPath` | `String` | The VDLROOT to use. If empty, a packaged VDLROOT will be used | `""` |

Typical users will only need to specify `inputPaths`, the defaults for the
other options will work for most projects.

## Tasks

The VDL plugin adds the `vdl` task to your project. It will also add VDL output
directories to the `clean` task if your project has one.

If you use it in conjunction with Android or Java plugins, it will make
additional changes to the project as described below.

### Java plugin

If you have the `java` plugin installed (for example, you are calling `apply
plugin: 'java'`), the VDL plugin will take some additional steps:

* generated VDL Java files are automatically added to the project's main source set
* the `compileJava` task will have a dependence on VDL generation added to it
* any VDL files found in any of the `inputPaths` will be added to the project's
  resources

### Android plugin

If you have either the `com.android.library` or `com.android.application`
plugins applied (e.g. by doing `apply plugin: 'com.android.application'`), the
VDL plugin will take some additional steps:

* generated VDL Java files are automatically added to the project's main
  Android source set
* the `preBuild` task will have a dependence on VDL generation added to it

## Libraries and dependencies

It is convenient to be able to refer to VDL files contained in libraries that
you depend on. This allows you to incorporate VDL types from these libraries in
your own VDL types. Similarly, if you are publishing a library that uses VDL,
it is desirable to allow your libraries consumers to use your VDL types in the
same fashion.

As mentioned above, if you use the `java` plugin, the VDL plugin will add VDL
files from your VDL `inputPaths` to your projects resources. This implies that
any JARs produced by your project will contain all of these VDL files.

If the VDL plugin encounters a library in the project's dependencies that
contains a JAR file built as described above, it will copy those VDL files into
its `transitiveVdlDir`. It will make those VDL files available to the VDL tool
so that VDL files in one of the `inputPaths` may refer to them.

[VDL option]: https://github.com/vanadium/go.ref/blob/master/cmd/vdl/main.go#L441
