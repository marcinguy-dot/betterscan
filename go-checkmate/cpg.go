package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	cpgVersion       = "10.8.2"
	cpgGroupID       = "de.fraunhofer.aisec"
	cpgArtifactID    = "cpg-core"
	minJavaVersion   = 17
	// Must match package + class in cpg/src/main/java/.../Runner.java
	cpgRunnerPackage = "com.checkmate.security.Runner"
)

var cpgRunnerSource = filepath.Join("cpg", "src", "main", "java", "com", "checkmate", "security", "Runner.java")

type mavenPom struct {
	Parent struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"parent"`
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Properties struct {
		Entries []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	} `xml:"properties"`
	DependencyManagement struct {
		Dependencies []mavenDependency `xml:"dependencies>dependency"`
	} `xml:"dependencyManagement"`
	Dependencies []mavenDependency `xml:"dependencies>dependency"`
	parentPom    *mavenPom
}

func (pom *mavenPom) getProperty(name string) string {
	switch name {
	case "project.version", "version", "pom.version":
		return pom.Version
	case "project.groupId", "groupId", "pom.groupId":
		return pom.GroupID
	case "project.artifactId", "artifactId", "pom.artifactId":
		return pom.ArtifactID
	}
	for _, entry := range pom.Properties.Entries {
		if entry.XMLName.Local == name {
			return entry.Value
		}
	}
	if pom.parentPom != nil {
		return pom.parentPom.getProperty(name)
	}
	return ""
}

func (pom *mavenPom) resolveValue(val string) string {
	for strings.Contains(val, "${") {
		start := strings.Index(val, "${")
		end := strings.Index(val[start:], "}")
		if end == -1 {
			break
		}
		end = start + end
		propName := val[start+2 : end]
		resolved := pom.getProperty(propName)
		if resolved == "" {
			break // avoid infinite loop if unresolved
		}
		val = val[:start] + resolved + val[end+1:]
	}
	return val
}

type mavenDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Type       string `xml:"type"`
	Scope      string `xml:"scope"`
	Optional   bool   `xml:"optional"`
}

type javaRuntime struct {
	JavaBin  string
	JavaHome string
	Note     string
}

func cpgHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".checkmate", "cpg", cpgVersion), nil
}

func bundledJavaHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".checkmate", "java", fmt.Sprintf("jdk-%d", minJavaVersion)), nil
}

func ensureJava(installMissing bool, requireJDK bool) (javaRuntime, error) {
	if javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME")); javaHome != "" {
		javaBin := filepath.Join(javaHome, "bin", javaBinaryName())
		if version, err := javaVersion(javaBin); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: javaBin, JavaHome: javaHome, Note: fmt.Sprintf("using JAVA_HOME (Java %d)", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	if path, err := exec.LookPath("java"); err == nil {
		if version, err := javaVersion(path); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: path, Note: fmt.Sprintf("using java from PATH (Java %d)", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	if bundledDir, err := bundledJavaHomeDir(); err == nil {
		javaBin := filepath.Join(bundledDir, "bin", javaBinaryName())
		if version, err := javaVersion(javaBin); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: javaBin, JavaHome: bundledDir, Note: fmt.Sprintf("using bundled JDK %d", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	if !installMissing {
		return javaRuntime{}, errors.New("Java 17+ is required for Fraunhofer CPG but was not found; re-run with --install-missing or install a JDK manually")
	}

	installMutex.Lock()
	defer installMutex.Unlock()

	// Recheck after lock to prevent double-downloading
	if javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME")); javaHome != "" {
		javaBin := filepath.Join(javaHome, "bin", javaBinaryName())
		if version, err := javaVersion(javaBin); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: javaBin, JavaHome: javaHome, Note: fmt.Sprintf("using JAVA_HOME (Java %d)", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	if path, err := exec.LookPath("java"); err == nil {
		if version, err := javaVersion(path); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: path, Note: fmt.Sprintf("using java from PATH (Java %d)", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	if bundledDir, err := bundledJavaHomeDir(); err == nil {
		javaBin := filepath.Join(bundledDir, "bin", javaBinaryName())
		if version, err := javaVersion(javaBin); err == nil && version >= minJavaVersion {
			runtime := javaRuntime{JavaBin: javaBin, JavaHome: bundledDir, Note: fmt.Sprintf("using bundled JDK %d", version)}
			if !requireJDK || hasJavac(runtime) {
				return runtime, nil
			}
		}
	}

	javaHome, note, err := downloadJavaJDK()
	if err != nil {
		return javaRuntime{}, fmt.Errorf("could not set up Java for Fraunhofer CPG: %w", err)
	}
	javaBin := filepath.Join(javaHome, "bin", javaBinaryName())
	version, err := javaVersion(javaBin)
	if err != nil || version < minJavaVersion {
		return javaRuntime{}, fmt.Errorf("downloaded Java setup failed validation (need Java %d+)", minJavaVersion)
	}
	return javaRuntime{JavaBin: javaBin, JavaHome: javaHome, Note: note}, nil
}

func hasJavac(java javaRuntime) bool {
	if java.JavaHome != "" {
		javacBin := filepath.Join(java.JavaHome, "bin", javacBinaryName())
		_, err := os.Stat(javacBin)
		return err == nil
	}
	_, err := exec.LookPath(javacBinaryName())
	return err == nil
}

func javaBinaryName() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

func javacBinaryName() string {
	if runtime.GOOS == "windows" {
		return "javac.exe"
	}
	return "javac"
}

func javaVersion(javaBin string) (int, error) {
	cmd := exec.Command(javaBin, "-version")
	output, err := cmd.CombinedOutput()
	text := string(output)
	if text == "" && err != nil {
		return 0, err
	}
	re := regexp.MustCompile(`version "(\d+)(?:\.|\+)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, fmt.Errorf("could not parse java version from: %s", strings.TrimSpace(text))
	}
	version, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, err
	}
	return version, nil
}

func downloadJavaJDK() (string, string, error) {
	osName, arch, err := adoptiumPlatform()
	if err != nil {
		return "", "", err
	}

	url := fmt.Sprintf(
		"https://api.adoptium.net/v3/assets/latest/%d/hotspot?os=%s&architecture=%s&image_type=jdk&vendor=eclipse",
		minJavaVersion, osName, arch,
	)
	resp, err := httpGetWithRetry(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to query Adoptium for Java %d (%s/%s): %w", minJavaVersion, osName, arch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Adoptium returned status %s for %s/%s", resp.Status, osName, arch)
	}

	var assets []struct {
		Binary struct {
			Package struct {
				Link string `json:"link"`
			} `json:"package"`
		} `json:"binary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return "", "", fmt.Errorf("failed to parse Adoptium response: %w", err)
	}
	packageLink := ""
	if len(assets) > 0 {
		packageLink = assets[0].Binary.Package.Link
	}
	if packageLink == "" {
		return "", "", fmt.Errorf("Adoptium has no JDK %d package for %s/%s", minJavaVersion, osName, arch)
	}

	targetHome, err := bundledJavaHomeDir()
	if err != nil {
		return "", "", err
	}
	if err := os.RemoveAll(targetHome); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetHome), 0o755); err != nil {
		return "", "", err
	}

	tempDir, err := os.MkdirTemp("", "checkmate-java-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "jdk-archive")
	if err := downloadFile(packageLink, archivePath); err != nil {
		return "", "", fmt.Errorf("failed to download JDK for %s/%s: %w", osName, arch, err)
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", "", err
	}
	if strings.HasSuffix(strings.ToLower(packageLink), ".zip") {
		if err := extractZip(archivePath, extractDir); err != nil {
			return "", "", err
		}
	} else {
		if err := extractTarGz(archivePath, extractDir); err != nil {
			return "", "", err
		}
	}

	jdkRoot, err := findJDKRoot(extractDir)
	if err != nil {
		return "", "", err
	}
	if err := os.Rename(jdkRoot, targetHome); err != nil {
		return "", "", err
	}

	note := fmt.Sprintf("installed Eclipse Temurin JDK %d for %s/%s", minJavaVersion, osName, arch)
	return targetHome, note, nil
}

func adoptiumPlatform() (string, string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "mac"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		return "", "", fmt.Errorf("unsupported OS for automatic Java install: %s", runtime.GOOS)
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "aarch64"
	default:
		return "", "", fmt.Errorf("unsupported architecture for automatic Java install: %s", runtime.GOARCH)
	}
	return osName, arch, nil
}

func findJDKRoot(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(root, entry.Name())
		javaBin := filepath.Join(candidate, "bin", javaBinaryName())
		if _, err := os.Stat(javaBin); err == nil {
			return candidate, nil
		}
		macHome := filepath.Join(candidate, "Contents", "Home")
		macJavaBin := filepath.Join(macHome, "bin", javaBinaryName())
		if _, err := os.Stat(macJavaBin); err == nil {
			return macHome, nil
		}
	}
	return "", errors.New("extracted JDK archive did not contain a bin/java executable")
}

func ensureCpgRuntime(context Context, java javaRuntime) (string, string, error) {
	home, err := cpgHomeDir()
	if err != nil {
		return "", "", err
	}
	libDir := filepath.Join(home, "lib")
	runnerClass := filepath.Join(home, "runner", strings.ReplaceAll(cpgRunnerPackage, ".", string(os.PathSeparator))+".class")

	jarsReady := false
	if entries, err := os.ReadDir(libDir); err == nil && len(entries) > 0 {
		jarsReady = true
	}
	classReady := false
	if _, err := os.Stat(runnerClass); err == nil {
		classReady = true
	}

	// Recompile runner when source is newer than the class (e.g. after detection fixes).
	if jarsReady {
		if src, srcErr := cpgRunnerSourcePath(); srcErr == nil {
			if needRecompileCpgRunner(src, runnerClass) {
				if note, compileErr := compileCpgRunner(java, home, src); compileErr == nil {
					return home, note, nil
				} else if !classReady {
					// Fall through to full install if we have no usable class.
					if !context.InstallMissing {
						return "", "", compileErr
					}
				} else {
					// Keep existing class if recompile failed but something is present.
					return home, "", nil
				}
			} else if classReady {
				return home, "", nil
			}
		} else if classReady {
			return home, "", nil
		}
	}

	if !context.InstallMissing {
		return "", "", errors.New("Fraunhofer CPG runtime is not installed; re-run with --install-missing")
	}

	installMutex.Lock()
	defer installMutex.Unlock()

	// Recheck after lock
	if _, err := os.Stat(runnerClass); err == nil {
		if entries, err := os.ReadDir(libDir); err == nil && len(entries) > 0 {
			if src, srcErr := cpgRunnerSourcePath(); srcErr == nil && needRecompileCpgRunner(src, runnerClass) {
				if note, compileErr := compileCpgRunner(java, home, src); compileErr == nil {
					return home, note, nil
				}
			}
			return home, "", nil
		}
	}

	note, err := installCpg(java)
	if err != nil {
		return "", "", err
	}
	return home, note, nil
}

func needRecompileCpgRunner(sourcePath, runnerClass string) bool {
	srcInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false
	}
	classInfo, err := os.Stat(runnerClass)
	if err != nil {
		return true
	}
	return srcInfo.ModTime().After(classInfo.ModTime())
}

func compileCpgRunner(java javaRuntime, home, sourcePath string) (string, error) {
	libDir := filepath.Join(home, "lib")
	runnerDir := filepath.Join(home, "runner")
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return "", err
	}

	javacBin := filepath.Join(java.JavaHome, "bin", javacBinaryName())
	if java.JavaHome == "" {
		if path, err := exec.LookPath("javac"); err == nil {
			javacBin = path
		} else {
			return "", errors.New("javac not found; a full JDK (not just JRE) is required to compile the CPG runner")
		}
	} else if _, err := os.Stat(javacBin); err != nil {
		return "", fmt.Errorf("javac not found at %s; a full JDK is required for Fraunhofer CPG", javacBin)
	}

	entries, err := os.ReadDir(libDir)
	if err != nil {
		return "", fmt.Errorf("failed to read lib directory: %w", err)
	}
	var classpathEntries []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar") {
			classpathEntries = append(classpathEntries, filepath.Join(libDir, entry.Name()))
		}
	}
	if len(classpathEntries) == 0 {
		return "", errors.New("no JAR files found in lib directory")
	}
	classpath := strings.Join(classpathEntries, string(os.PathListSeparator))
	compileCmd := exec.Command(javacBin, "-proc:none", "-cp", classpath, "-d", runnerDir, sourcePath)
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(compileOutput))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("failed to compile CPG runner: %s", msg)
	}
	return "recompiled Fraunhofer CPG runner", nil
}

func installCpg(java javaRuntime) (string, error) {
	home, err := cpgHomeDir()
	if err != nil {
		return "", err
	}
	libDir := filepath.Join(home, "lib")
	runnerDir := filepath.Join(home, "runner")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return "", err
	}

	// Download required CPG modules: cpg-core, cpg-analysis, and cpg-language-java
	modules := []string{"cpg-core", "cpg-analysis", "cpg-language-java"}
	for _, module := range modules {
		if err := resolveMavenArtifact(cpgGroupID, module, cpgVersion, libDir); err != nil {
			return "", fmt.Errorf("failed to download Fraunhofer CPG module %s: %w", module, err)
		}
	}

	sourcePath, err := cpgRunnerSourcePath()
	if err != nil {
		return "", err
	}

	if _, err := compileCpgRunner(java, home, sourcePath); err != nil {
		return "", err
	}

	return fmt.Sprintf("installed Fraunhofer CPG %s and compiled runner", cpgVersion), nil
}

func cpgRunnerSourcePath() (string, error) {
	execPath, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), cpgRunnerSource)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(wd, cpgRunnerSource)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("CPG runner source not found at %s", candidate)
}

func buildCpgCommand(context Context) (*exec.Cmd, string, error) {
	java, err := ensureJava(context.InstallMissing, true)
	if err != nil {
		return nil, "", err
	}

	home, installNote, err := ensureCpgRuntime(context, java)
	if err != nil {
		return nil, "", err
	}

	libDir := filepath.Join(home, "lib")
	runnerDir := filepath.Join(home, "runner")

	// Build classpath with all JAR files in libDir
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read lib directory: %w", err)
	}
	var classpathEntries []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jar") {
			classpathEntries = append(classpathEntries, filepath.Join(libDir, entry.Name()))
		}
	}
	if len(classpathEntries) == 0 {
		return nil, "", errors.New("no JAR files found in lib directory")
	}
	classpathEntries = append(classpathEntries, runnerDir)
	classpath := strings.Join(classpathEntries, string(os.PathListSeparator))

	codeDir, err := filepath.Abs(context.CodeDir)
	if err != nil {
		return nil, "", err
	}

	args := []string{
		"-cp", classpath,
		cpgRunnerPackage,
		codeDir,
	}
	cmd := exec.Command(java.JavaBin, args...)
	cmd.Dir = context.CodeDir
	if java.JavaHome != "" {
		cmd.Env = append(os.Environ(), "JAVA_HOME="+java.JavaHome)
	}

	notes := []string{}
	if java.Note != "" {
		notes = append(notes, java.Note)
	}
	if installNote != "" {
		notes = append(notes, installNote)
	}
	return cmd, strings.Join(notes, "; "), nil
}

func parseCpg(output []byte, analyzer string) []Issue {
	var payload struct {
		Findings []struct {
			Code    string `json:"code"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Message string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		if blob := extractJSONBlob(output); blob != nil {
			if err := json.Unmarshal(blob, &payload); err != nil {
				return nil
			}
		} else {
			return nil
		}
	}

	var issues []Issue
	for _, item := range payload.Findings {
		code := item.Code
		if code == "" {
			code = "CPG_FINDING"
		}
		issues = append(issues, newIssue(analyzer, code, item.File, item.Line, item.Message))
	}
	return issues
}

func resolveMavenArtifact(groupID, artifactID, version, destDir string) error {
	seen := make(map[string]struct{})
	var resolve func(groupID, artifactID, version string) error
	resolve = func(groupID, artifactID, version string) error {
		key := groupID + ":" + artifactID + ":" + version
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}

		pom, err := fetchMavenPom(groupID, artifactID, version)
		if err != nil {
			return err
		}

		if pom.GroupID == "" {
			pom.GroupID = groupID
		}
		if pom.ArtifactID == "" {
			pom.ArtifactID = artifactID
		}
		if pom.Version == "" {
			pom.Version = version
		}

		if err := downloadMavenJar(pom.GroupID, pom.ArtifactID, pom.Version, destDir); err != nil {
			return err
		}

		managed := make(map[string]string)
		for _, dep := range pom.DependencyManagement.Dependencies {
			if dep.Version != "" {
				depGroupID := pom.resolveValue(dep.GroupID)
				depArtifactID := pom.resolveValue(dep.ArtifactID)
				managed[depGroupID+":"+depArtifactID] = pom.resolveValue(dep.Version)
			}
		}

		for _, dep := range pom.Dependencies {
			if dep.Optional || !mavenScopeIncluded(dep.Scope) {
				continue
			}
			depGroupID := pom.resolveValue(dep.GroupID)
			depArtifactID := pom.resolveValue(dep.ArtifactID)
			depVersion := pom.resolveValue(dep.Version)
			if depVersion == "" || strings.Contains(depVersion, "${") {
				keyResolved := depGroupID + ":" + depArtifactID
				keyUnresolved := dep.GroupID + ":" + dep.ArtifactID
				if managedVersion, ok := managed[keyResolved]; ok {
					depVersion = managedVersion
				} else if managedVersion, ok := managed[keyUnresolved]; ok {
					depVersion = managedVersion
				}
			}
			if depVersion == "" || strings.Contains(depVersion, "${") {
				continue
			}
			if err := resolve(depGroupID, depArtifactID, depVersion); err != nil {
				return err
			}
		}
		return nil
	}

	return resolve(groupID, artifactID, version)
}

func mavenScopeIncluded(scope string) bool {
	switch strings.TrimSpace(scope) {
	case "", "compile", "runtime":
		return true
	default:
		return false
	}
}

func fetchMavenPom(groupID, artifactID, version string) (*mavenPom, error) {
	url := mavenArtifactURL(groupID, artifactID, version) + ".pom"
	resp, err := httpGetWithRetry(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("maven pom download failed for %s:%s:%s (%s)", groupID, artifactID, version, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pom mavenPom
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := dec.Decode(&pom); err != nil {
		return nil, err
	}

	if pom.Parent.GroupID != "" && pom.Parent.ArtifactID != "" && pom.Parent.Version != "" {
		parentPom, err := fetchMavenPom(pom.Parent.GroupID, pom.Parent.ArtifactID, pom.Parent.Version)
		if err == nil {
			pom.parentPom = parentPom
			for _, dep := range parentPom.DependencyManagement.Dependencies {
				pom.DependencyManagement.Dependencies = append(pom.DependencyManagement.Dependencies, dep)
			}
		}
	}

	if pom.GroupID == "" {
		pom.GroupID = groupID
	}
	if pom.ArtifactID == "" {
		pom.ArtifactID = artifactID
	}
	if pom.Version == "" {
		pom.Version = version
	}
	return &pom, nil
}

func downloadMavenJar(groupID, artifactID, version, destDir string) error {
	fileName := fmt.Sprintf("%s-%s.jar", artifactID, version)
	target := filepath.Join(destDir, fileName)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	url := mavenArtifactURL(groupID, artifactID, version) + ".jar"
	return downloadFile(url, target)
}

func mavenArtifactURL(groupID, artifactID, version string) string {
	groupPath := strings.ReplaceAll(groupID, ".", "/")
	return fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s-%s", groupPath, artifactID, version, artifactID, version)
}

func downloadFile(url, target string) error {
	resp, err := httpGetWithRetry(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download failed (%s): %s", resp.Status, url)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".download-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target := filepath.Join(destDir, file.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid zip entry: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return err
		}
		dst.Close()
		src.Close()
	}
	return nil
}

func hasSupportedCpgSources(codeDir string) (bool, error) {
	supported := map[string]struct{}{
		".java": {}, ".c": {}, ".cc": {}, ".cpp": {}, ".cxx": {}, ".h": {}, ".hpp": {},
		".go": {}, ".py": {}, ".rb": {}, ".ts": {}, ".js": {}, ".tsx": {}, ".jsx": {},
		".ll": {}, ".ini": {},
	}
	found := false
	err := filepath.WalkDir(codeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".checkmate" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := supported[strings.ToLower(filepath.Ext(path))]; ok {
			found = true
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return found, nil
}

func installCpgTool() (string, error) {
	java, err := ensureJava(true, true)
	if err != nil {
		return "", err
	}
	installMutex.Lock()
	defer installMutex.Unlock()
	return installCpg(java)
}

func cpgInstallPreflight() error {
	if _, _, err := adoptiumPlatform(); err != nil {
		return fmt.Errorf("Fraunhofer CPG requires Java %d+ but automatic JDK download is not supported on %s/%s: %w", minJavaVersion, runtime.GOOS, runtime.GOARCH, err)
	}
	return nil
}

func cpgJSONFromMixedOutput(stdout, stderr []byte) []byte {
	combined := append(append([]byte{}, stdout...), stderr...)
	start := bytes.IndexByte(combined, '{')
	if start == -1 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(combined[start:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil
	}
	return raw
}

func httpGetWithRetry(url string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var resp *http.Response
	var err error
	backoff := 500 * time.Millisecond

	for i := 0; i < 5; i++ {
		req, reqErr := http.NewRequest("GET", url, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", "checkmate-go/1.0.0 (https://github.com/marcinguy/checkmate-go)")

		resp, err = client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
				resp.Body.Close()
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return resp, nil
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	if err != nil {
		return nil, err
	}
	return resp, nil
}
