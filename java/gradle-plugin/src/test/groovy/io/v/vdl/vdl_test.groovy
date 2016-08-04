import com.google.common.base.Charsets
import com.google.common.io.Files
import io.v.vdl.VdlConfiguration
import io.v.vdl.VdlPlugin
import org.gradle.api.Project
import org.gradle.api.internal.ClosureBackedAction
import org.gradle.testfixtures.ProjectBuilder
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

import java.util.jar.JarEntry
import java.util.jar.JarOutputStream
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

import static com.google.common.truth.Truth.assertThat

class VdlPluginTest {
    @Rule
    public TemporaryFolder temporaryFolder = new TemporaryFolder();

    @Test
    public void vdlPluginTest() {
        Project project = ProjectBuilder.builder().build()

        createVdlToolJar(new File(project.getProjectDir(), "test-gradle-plugin-vdltool-arch1.jar"), "vdl-somearch",
                "blahface")
        createVdlToolJar(new File(project.getProjectDir(), "test-gradle-plugin-vdltool-arch2.jar"), "vdl-macosx",
                "hello")

        List<File> vdlToolFiles = VdlPlugin.getVdlToolJarFiles(project,
                project.files(
                        'test-gradle-plugin-vdltool-arch1.jar',
                        'test-gradle-plugin-vdltool-arch2.jar'));
        assertThat(vdlToolFiles).named('VDL jar files').isNotEmpty()

        Map<String, String> fileContents = new HashMap<String, String>()
        // Read all the files
        for (File f : vdlToolFiles) {
            fileContents.put(f.getName(), Files.toString(f, Charsets.US_ASCII))
        }

        assertThat(fileContents).hasSize(2)
        assertThat(fileContents).containsEntry("vdl-somearch", "blahface")
        assertThat(fileContents).containsEntry("vdl-macosx", "hello")
    }

    @Test
    public void multipleVdlDependencies() {
        Project rootProject = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder()).withName('root')
                .build()

        // Create a VDL project with no dependencies.
        Project vdlProjectA = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectA').build()
        vdlProjectA.pluginManager.apply('java')
        // Create a fake VDL file in the project's source directory.
        File sourceDir = new File(vdlProjectA.getProjectDir(), 'src/main/java')
        assertThat(sourceDir.mkdirs()).isTrue()
        assertThat(new File(sourceDir, "projectA.vdl").createNewFile()).isTrue()
        vdlProjectA.pluginManager.apply(VdlPlugin.class)
        vdlProjectA.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += sourceDir.getPath()
        }))

        // Create a jar file with some VDL files in it.
        File jarFile = new File(rootProject.getProjectDir(), 'mydep.jar')
        ZipOutputStream stream = new ZipOutputStream(new FileOutputStream(jarFile))
        byte[] output = "hello world".getBytes(Charsets.US_ASCII)
        stream.putNextEntry(new JarEntry('whatever/jar.vdl'))
        stream.write(output)
        stream.closeEntry()
        stream.flush()
        stream.close()

        Project vdlProjectB = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectB').build()
        vdlProjectB.pluginManager.apply(VdlPlugin.class)
        vdlProjectB.pluginManager.apply('java')
        vdlProjectB.repositories.flatDir(dirs: jarFile.getParent())
        vdlProjectB.dependencies.add('compile', rootProject.files(jarFile.getAbsolutePath()))
        vdlProjectB.dependencies.add('compile', vdlProjectA)
        vdlProjectB.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += 'src/main/java'
            outputPath = 'generated-src/vdl'
        }))

        Set<String> inputPaths = VdlPlugin.extractTransitiveVdlFilesAndGetInputPaths(vdlProjectB)

        // vdlProjectB should now have two VDLPATH elements:
        //   - generated-src/transitive-vdl, containing whatever/jar.vdl and projectA.vdl
        //   - src/main/java, containing no vdl files
        assertThat(inputPaths).containsExactly('generated-src/transitive-vdl', 'src/main/java')
        assertThat(new File(vdlProjectB.getProjectDir(), 'generated-src/transitive-vdl/whatever/jar.vdl').exists())
                .isTrue()
        assertThat(new File(vdlProjectB.getProjectDir(), 'generated-src/transitive-vdl/projectA.vdl').exists()).isTrue()
        assertThat(VdlPlugin.getJavaOutDirs(vdlProjectB)).containsExactly('src/main/java->generated-src/vdl',
                vdlProjectB.vdl.transitiveVdlDir + '->' + vdlProjectB.vdl.transitiveVdlDir)
    }

    @Test
    public void transitiveVdlDependencies() {
        Project rootProject = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder()).withName('root')
                .build()

        // Create a VDL project with no dependencies.
        Project vdlProjectA = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectA').build()
        vdlProjectA.pluginManager.apply('java')
        // Create a fake VDL file in the project's source directory.
        File sourceDir = new File(vdlProjectA.getProjectDir(), 'src/main/java')
        assertThat(sourceDir.mkdirs()).isTrue()
        assertThat(new File(sourceDir, "projectA.vdl").createNewFile()).isTrue()
        vdlProjectA.pluginManager.apply(VdlPlugin.class)
        vdlProjectA.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += sourceDir.getPath()
        }))

        // Create a regular (non-VDL) project B that depends on VDL project A.
        Project regularProjectB = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('regularProjectB').build()
        regularProjectB.pluginManager.apply('java')
        regularProjectB.dependencies.add('compile', vdlProjectA)

        // Create a VDL project that depends on project B and should therefore transitively depend on A's VDL files.
        Project vdlProjectC = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectC').build()
        vdlProjectC.pluginManager.apply('java')
        vdlProjectC.pluginManager.apply(VdlPlugin.class)
        vdlProjectC.dependencies.add('compile', regularProjectB)
        vdlProjectC.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += 'some/directory'
        }))
        vdlProjectC.evaluate()

        Set<String> inputPaths = VdlPlugin.extractTransitiveVdlFilesAndGetInputPaths(vdlProjectC)

        // vdlProjectC should now have two VDLPATH elements:
        //   - generated-src/transitive-vdl, containing projectA.vdl
        //   - some/directory, containing no vdl files
        assertThat(inputPaths).containsExactly('generated-src/transitive-vdl', 'some/directory')
        assertThat(new File(vdlProjectC.getProjectDir(), 'generated-src/transitive-vdl/projectA.vdl').exists()).isTrue()
        assertThat(VdlPlugin.getJavaOutDirs(vdlProjectC)).containsExactly('some/directory->generated-src/vdl',
                vdlProjectC.vdl.transitiveVdlDir + '->' + vdlProjectC.vdl.transitiveVdlDir)
    }

    @Test
    public void determineVdlRoot() {
        Project rootProject = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder()).withName('root')
                .build()

        // Create a VDL project with no dependencies.
        Project vdlProjectA = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectA').build()
        vdlProjectA.pluginManager.apply('java')
        // Create a fake VDL file in the project's source directory.
        File sourceDir = new File(vdlProjectA.getProjectDir(), 'release/go/src/v.io/v23/vdlroot')
        assertThat(sourceDir.mkdirs()).isTrue()
        assertThat(new File(sourceDir, "root.vdl").createNewFile()).isTrue()
        vdlProjectA.pluginManager.apply(VdlPlugin.class)
        vdlProjectA.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += 'release/go/src'
        }))

        // Create a VDL project that depends on project B and should therefore have the vdlroot in
        // its transitive VDL files.
        Project vdlProjectB = ProjectBuilder.builder().withProjectDir(temporaryFolder.newFolder())
                .withParent(rootProject).withName('vdlProjectB').build()
        vdlProjectB.pluginManager.apply('java')
        vdlProjectB.pluginManager.apply(VdlPlugin.class)
        vdlProjectB.dependencies.add('compile', vdlProjectA)
        vdlProjectB.extensions.configure(VdlConfiguration, new ClosureBackedAction<VdlConfiguration>({
            inputPaths += 'some/directory'
        }))
        vdlProjectB.evaluate()

        VdlPlugin.extractTransitiveVdlFilesAndGetInputPaths(vdlProjectB)

        // vdlProjectB's VDLROOT should be set to the vdlroot inside the transitive-vdl dir.
        assertThat(VdlPlugin.getVdlRootPath(vdlProjectB)).isEqualTo(new File(vdlProjectB.getProjectDir(),
                '/generated-src/transitive-vdl/v.io/v23/vdlroot').getAbsolutePath())
        assertThat(new File(vdlProjectB.getProjectDir(), 'generated-src/transitive-vdl/v.io/v23/vdlroot')
                .isDirectory()).isTrue()
    }

    private static void createVdlToolJar(File outputFile, String entryName, String vdlBinContents) {
        JarOutputStream outputStream = new JarOutputStream(new FileOutputStream(outputFile))
        outputStream.putNextEntry(new ZipEntry(entryName))
        outputStream.write(vdlBinContents.getBytes(Charsets.US_ASCII))
        outputStream.close()
    }
}