package cobertura

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/coverctl/internal/application"
)

func TestParser_Format(t *testing.T) {
	p := New()
	assert.Equal(t, application.FormatCobertura, p.Format())
}

func TestParser_Parse_ValidCobertura(t *testing.T) {
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example">
      <classes>
        <class name="Main" filename="src/Main.java">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="1"/>
            <line number="3" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 2, stats["src/Main.java"].Covered)
	assert.Equal(t, 3, stats["src/Main.java"].Total)
}

func TestParser_Parse_MultiplePackages(t *testing.T) {
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example.core">
      <classes>
        <class name="Core" filename="src/core/Core.java">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="1"/>
          </lines>
        </class>
      </classes>
    </package>
    <package name="com.example.util">
      <classes>
        <class name="Utils" filename="src/util/Utils.java">
          <lines>
            <line number="1" hits="0"/>
            <line number="2" hits="0"/>
            <line number="3" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 2, stats["src/core/Core.java"].Covered)
	assert.Equal(t, 2, stats["src/core/Core.java"].Total)
	assert.Equal(t, 0, stats["src/util/Utils.java"].Covered)
	assert.Equal(t, 3, stats["src/util/Utils.java"].Total)
}

func TestParser_Parse_MultipleClassesSameFile(t *testing.T) {
	// Some formats have multiple classes per file (inner classes, etc.)
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example">
      <classes>
        <class name="Main" filename="src/Main.java">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="1"/>
          </lines>
        </class>
        <class name="Main$Inner" filename="src/Main.java">
          <lines>
            <line number="10" hits="1"/>
            <line number="11" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	// Both classes contribute to the same file
	assert.Equal(t, 3, stats["src/Main.java"].Covered)
	assert.Equal(t, 4, stats["src/Main.java"].Total)
}

func TestParser_Parse_WithMethods(t *testing.T) {
	// Some Cobertura variants nest lines under methods
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="mypackage">
      <classes>
        <class name="MyClass" filename="mypackage/myclass.py">
          <methods>
            <method name="__init__">
              <lines>
                <line number="1" hits="5"/>
                <line number="2" hits="5"/>
              </lines>
            </method>
            <method name="process">
              <lines>
                <line number="5" hits="3"/>
                <line number="6" hits="0"/>
              </lines>
            </method>
          </methods>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, 3, stats["mypackage/myclass.py"].Covered)
	assert.Equal(t, 4, stats["mypackage/myclass.py"].Total)
}

func TestParser_Parse_EmptyPackages(t *testing.T) {
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestParser_Parse_NoFilename(t *testing.T) {
	// Classes without filename should be skipped
	content := `<?xml version="1.0"?>
<coverage version="1.0">
  <packages>
    <package name="com.example">
      <classes>
        <class name="NoFile">
          <lines>
            <line number="1" hits="1"/>
          </lines>
        </class>
        <class name="WithFile" filename="src/WithFile.java">
          <lines>
            <line number="1" hits="1"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Contains(t, stats, "src/WithFile.java")
}

func TestParser_Parse_FileNotFound(t *testing.T) {
	parser := New()
	_, err := parser.Parse("/nonexistent/path/coverage.xml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "open cobertura file")
}

func TestParser_Parse_InvalidXML(t *testing.T) {
	content := `this is not xml`

	tmpfile := createTempFile(t, content)

	parser := New()
	_, err := parser.Parse(tmpfile)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode cobertura xml")
}

func TestParser_ParseAll_MergesProfiles(t *testing.T) {
	content1 := `<?xml version="1.0"?>
<coverage>
  <packages>
    <package name="pkg1">
      <classes>
        <class filename="src/a.py">
          <lines>
            <line number="1" hits="1"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	content2 := `<?xml version="1.0"?>
<coverage>
  <packages>
    <package name="pkg2">
      <classes>
        <class filename="src/b.py">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="0"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`

	tmpfile1 := createTempFile(t, content1)
	tmpfile2 := createTempFile(t, content2)

	parser := New()
	stats, err := parser.ParseAll([]string{tmpfile1, tmpfile2})

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 1, stats["src/a.py"].Covered)
	assert.Equal(t, 1, stats["src/b.py"].Covered)
}

func TestParser_ParseAll_EmptyPaths(t *testing.T) {
	parser := New()
	stats, err := parser.ParseAll([]string{})

	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestParser_Parse_RealWorldPython(t *testing.T) {
	// Simulated coverage.py XML output
	content := `<?xml version="1.0" ?>
<coverage version="5.5" timestamp="1234567890" lines-valid="100" lines-covered="75" line-rate="0.75" branches-valid="0" branches-covered="0" branch-rate="0" complexity="0">
    <packages>
        <package name="myapp" line-rate="0.8" branch-rate="0" complexity="0">
            <classes>
                <class name="__init__.py" filename="myapp/__init__.py" line-rate="1" branch-rate="0" complexity="0">
                    <lines>
                        <line number="1" hits="1"/>
                        <line number="2" hits="1"/>
                    </lines>
                </class>
                <class name="main.py" filename="myapp/main.py" line-rate="0.75" branch-rate="0" complexity="0">
                    <lines>
                        <line number="1" hits="1"/>
                        <line number="2" hits="1"/>
                        <line number="3" hits="1"/>
                        <line number="4" hits="0"/>
                    </lines>
                </class>
            </classes>
        </package>
    </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 2)
	assert.Equal(t, 2, stats["myapp/__init__.py"].Covered)
	assert.Equal(t, 2, stats["myapp/__init__.py"].Total)
	assert.Equal(t, 3, stats["myapp/main.py"].Covered)
	assert.Equal(t, 4, stats["myapp/main.py"].Total)
}

func TestParser_Parse_RealWorldJava(t *testing.T) {
	// Simulated JaCoCo Cobertura XML output
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
<coverage line-rate="0.85" branch-rate="0.75" lines-covered="85" lines-valid="100" branches-covered="30" branches-valid="40" complexity="0" version="1.0" timestamp="1234567890">
    <sources>
        <source>/home/user/project/src/main/java</source>
    </sources>
    <packages>
        <package name="com.example.service" line-rate="0.9" branch-rate="0.8" complexity="0">
            <classes>
                <class name="com.example.service.UserService" filename="com/example/service/UserService.java" line-rate="0.9" branch-rate="0.8" complexity="0">
                    <methods>
                        <method name="findUser" signature="(Ljava/lang/String;)Lcom/example/model/User;" line-rate="1.0" branch-rate="1.0" complexity="1">
                            <lines>
                                <line number="15" hits="10"/>
                                <line number="16" hits="10"/>
                                <line number="17" hits="10"/>
                            </lines>
                        </method>
                        <method name="deleteUser" signature="(Ljava/lang/String;)V" line-rate="0.5" branch-rate="0.5" complexity="2">
                            <lines>
                                <line number="25" hits="5"/>
                                <line number="26" hits="0"/>
                            </lines>
                        </method>
                    </methods>
                    <lines>
                        <line number="10" hits="1"/>
                    </lines>
                </class>
            </classes>
        </package>
    </packages>
</coverage>`

	tmpfile := createTempFile(t, content)

	parser := New()
	stats, err := parser.Parse(tmpfile)

	require.NoError(t, err)
	require.Len(t, stats, 1)

	// Class has 1 direct line + 5 method lines = 6 total
	// 1 + 3 (findUser) + 1 (deleteUser first line) = 5 covered
	stat := stats["com/example/service/UserService.java"]
	assert.Equal(t, 5, stat.Covered)
	assert.Equal(t, 6, stat.Total)
}

// createTempFile creates a temporary file with the given content.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpdir := t.TempDir()
	tmpfile := filepath.Join(tmpdir, "coverage.xml")
	err := os.WriteFile(tmpfile, []byte(content), 0o644)
	require.NoError(t, err)
	return tmpfile
}
