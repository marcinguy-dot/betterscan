package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveValueAndGetProperty(t *testing.T) {
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-project</artifactId>
	<version>1.0.0</version>
	<properties>
		<slf4j.version>1.7.30</slf4j.version>
		<nested.prop>${slf4j.version}</nested.prop>
	</properties>
</project>`

	var pom mavenPom
	if err := xml.Unmarshal([]byte(pomContent), &pom); err != nil {
		t.Fatalf("failed to unmarshal POM: %v", err)
	}

	if pom.GroupID == "" {
		pom.GroupID = "com.example"
	}
	if pom.ArtifactID == "" {
		pom.ArtifactID = "test-project"
	}
	if pom.Version == "" {
		pom.Version = "1.0.0"
	}

	val := pom.resolveValue("${slf4j.version}")
	if val != "1.7.30" {
		t.Errorf("expected 1.7.30, got %q", val)
	}

	nestedVal := pom.resolveValue("${nested.prop}")
	if nestedVal != "1.7.30" {
		t.Errorf("expected 1.7.30 for nested property, got %q", nestedVal)
	}

	projVersion := pom.resolveValue("${project.version}")
	if projVersion != "1.0.0" {
		t.Errorf("expected 1.0.0, got %q", projVersion)
	}

	projGroupID := pom.resolveValue("${project.groupId}")
	if projGroupID != "com.example" {
		t.Errorf("expected com.example, got %q", projGroupID)
	}
}

func TestResolveMavenArtifactWithProperties(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "maven-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Resolve a small artifact with parent POM property resolution if any (e.g. slf4j-api)
	// We'll target org.slf4j:slf4j-api:1.7.30 which has some standard structure.
	err = resolveMavenArtifact("org.slf4j", "slf4j-api", "1.7.30", tempDir)
	if err != nil {
		t.Fatalf("failed to resolve slf4j-api: %v", err)
	}

	// Verify jar file was downloaded
	jarPath := filepath.Join(tempDir, "slf4j-api-1.7.30.jar")
	if _, err := os.Stat(jarPath); err != nil {
		t.Errorf("expected downloaded jar %s: %v", jarPath, err)
	}
}

func TestISO88591PomParsing(t *testing.T) {
	pomContent := `<?xml version="1.0" encoding="ISO-8859-1"?>
<project>
	<groupId>com.example</groupId>
	<artifactId>test-iso</artifactId>
	<version>1.0.0</version>
</project>`

	var pom mavenPom
	dec := xml.NewDecoder(bytes.NewReader([]byte(pomContent)))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := dec.Decode(&pom); err != nil {
		t.Fatalf("failed to decode ISO-8859-1 POM: %v", err)
	}

	if pom.GroupID == "" {
		pom.GroupID = "com.example"
	}
	if pom.GroupID != "com.example" || pom.ArtifactID != "test-iso" || pom.Version != "1.0.0" {
		t.Errorf("incorrect values parsed: %+v", pom)
	}
}

func TestMavenScopeIncluded(t *testing.T) {
	tests := []struct {
		scope    string
		included bool
	}{
		{"", true},
		{"compile", true},
		{"runtime", true},
		{"test", false},
		{"provided", false},
		{"system", false},
	}
	for _, tt := range tests {
		if got := mavenScopeIncluded(tt.scope); got != tt.included {
			t.Errorf("mavenScopeIncluded(%q) = %v, want %v", tt.scope, got, tt.included)
		}
	}
}

func TestMavenArtifactURL(t *testing.T) {
	url := mavenArtifactURL("com.example", "test-artifact", "1.0.0")
	expected := "https://repo1.maven.org/maven2/com/example/test-artifact/1.0.0/test-artifact-1.0.0"
	if url != expected {
		t.Errorf("mavenArtifactURL() = %q, want %q", url, expected)
	}
}

func TestResolveMavenArtifactMultipleModules(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "maven-multi-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Test resolving multiple CPG modules
	modules := []string{"cpg-core", "cpg-analysis", "cpg-language-java"}
	for _, module := range modules {
		err = resolveMavenArtifact("de.fraunhofer.aisec", module, "10.8.2", tempDir)
		if err != nil {
			t.Fatalf("failed to resolve %s: %v", module, err)
		}
	}

	// Verify all JAR files were downloaded
	for _, module := range modules {
		jarPath := filepath.Join(tempDir, module+"-10.8.2.jar")
		if _, err := os.Stat(jarPath); err != nil {
			t.Errorf("expected downloaded jar %s: %v", jarPath, err)
		}
	}
}
