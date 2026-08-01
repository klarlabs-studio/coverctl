package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	// Create a temporary directory for testing symlinks
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "realfile.txt")
	if err := os.WriteFile(realFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{
			name:    "empty path returns ErrEmptyPath",
			path:    "",
			wantErr: ErrEmptyPath,
		},
		{
			name:    "path with null byte returns ErrNullBytes",
			path:    "some\x00path",
			wantErr: ErrNullBytes,
		},
		{
			name:    "valid relative path succeeds",
			path:    "some/valid/path.txt",
			wantErr: nil,
		},
		{
			name:    "valid absolute path succeeds",
			path:    "/some/valid/path.txt",
			wantErr: nil,
		},
		{
			name:    "path with dot-dot is cleaned",
			path:    "some/../valid/path.txt",
			wantErr: nil,
		},
		{
			name:    "path with single dot is cleaned",
			path:    "./some/./path.txt",
			wantErr: nil,
		},
		{
			name:    "symlink is resolved",
			path:    symlinkPath,
			wantErr: nil,
		},
		{
			name:    "non-existent path returns cleaned path",
			path:    filepath.Join(tmpDir, "nonexistent", "file.txt"),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(tt.path)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidatePath(%q) unexpected error: %v", tt.path, err)
				return
			}

			if result == "" {
				t.Errorf("ValidatePath(%q) returned empty string", tt.path)
			}
		})
	}
}

func TestValidatePath_SymlinkResolution(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "realfile.txt")
	if err := os.WriteFile(realFile, []byte("test"), 0o600); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	result, err := ValidatePath(symlinkPath)
	if err != nil {
		t.Fatalf("ValidatePath(%q) error: %v", symlinkPath, err)
	}

	// Result should be the resolved real path, not the symlink
	// Use EvalSymlinks on the expected path too to handle OS-level symlinks (e.g., /var -> /private/var on macOS)
	expectedPath, err := filepath.EvalSymlinks(realFile)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error: %v", realFile, err)
	}

	if result != expectedPath {
		t.Errorf("ValidatePath(%q) = %q, want %q (resolved symlink)", symlinkPath, result, expectedPath)
	}
}

func TestValidatePath_NullByteVariants(t *testing.T) {
	nullPaths := []string{
		"\x00",
		"path\x00",
		"\x00path",
		"pa\x00th",
		"/some/\x00/path",
		"some/path\x00.txt",
	}

	for _, path := range nullPaths {
		t.Run("null_in_path", func(t *testing.T) {
			_, err := ValidatePath(path)
			if err != ErrNullBytes {
				t.Errorf("ValidatePath(%q) error = %v, want ErrNullBytes", path, err)
			}
		})
	}
}

func TestValidateScopedPath_AcceptsInScope(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub", "dir"), 0o750); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cases := []string{
		"file.txt",
		"sub/file.txt",
		"sub/dir/file.txt",
		"./sub/file.txt",
		"sub/../file.txt", // escapes "sub" but stays in root
		"sub/dir/../file.txt",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			got, err := ValidateScopedPath(p, root)
			if err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("expected absolute path, got %q", got)
			}
		})
	}
}

func TestValidateScopedPath_RejectsOutOfScope(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"null byte", "foo\x00bar"},
		{"absolute", "/etc/passwd"},
		{"home expansion attempt", "~/.ssh/authorized_keys"},
		{"parent escape", "../etc/passwd"},
		{"deep parent escape", "../../etc/passwd"},
		{"interior parent escape", "sub/../../../etc/passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateScopedPath(tc.path, root)
			if err == nil {
				t.Fatalf("expected error for %q", tc.path)
			}
		})
	}
}

func TestValidateScopedPath_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	link := filepath.Join(root, "evil-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err := ValidateScopedPath("evil-link", root)
	if err == nil {
		t.Fatal("expected rejection of symlink that escapes root")
	}
	if err != ErrPathEscapesBase {
		t.Errorf("expected ErrPathEscapesBase, got %v", err)
	}
}

func TestValidateScopedPath_PrefixCollisionGuard(t *testing.T) {
	// Ensures /foo/barbaz is not accepted when root is /foo/bar.
	parent := t.TempDir()
	root := filepath.Join(parent, "bar")
	sibling := filepath.Join(parent, "barbaz")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("setup root: %v", err)
	}
	if err := os.MkdirAll(sibling, 0o750); err != nil {
		t.Fatalf("setup sibling: %v", err)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(sibling, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := ValidateScopedPath("link", root); err == nil {
		t.Error("expected rejection — symlink target is sibling of root, not inside")
	}
}

func TestIsPathSafe(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "empty path is not safe",
			path: "",
			want: false,
		},
		{
			name: "null byte is not safe",
			path: "some\x00path",
			want: false,
		},
		{
			name: "valid relative path is safe",
			path: "some/valid/path.txt",
			want: true,
		},
		{
			name: "valid absolute path is safe",
			path: "/some/valid/path.txt",
			want: true,
		},
		{
			name: "parent traversal at start is not safe",
			path: "../some/path",
			want: false,
		},
		{
			name: "parent traversal after clean is not safe",
			path: "a/../../path",
			want: false,
		},
		{
			name: "contained parent traversal is safe",
			path: "some/../valid/path",
			want: true,
		},
		{
			name: "single dot path is safe",
			path: "./path",
			want: true,
		},
		{
			name: "current directory is safe",
			path: ".",
			want: true,
		},
		{
			name: "double slash path is safe",
			path: "some//path",
			want: true,
		},
		{
			name: "path with spaces is safe",
			path: "some path/with spaces.txt",
			want: true,
		},
		{
			name: "hidden file is safe",
			path: ".hidden/file.txt",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathSafe(tt.path); got != tt.want {
				t.Errorf("IsPathSafe(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsPathSafe_NullByteVariants(t *testing.T) {
	nullPaths := []string{
		"\x00",
		"path\x00",
		"\x00path",
		"pa\x00th",
		"/some/\x00/path",
		"some/path\x00.txt",
	}

	for _, path := range nullPaths {
		t.Run("null_in_path", func(t *testing.T) {
			if IsPathSafe(path) {
				t.Errorf("IsPathSafe(%q) = true, want false (contains null byte)", path)
			}
		})
	}
}

func TestValidatePath_CleanedPath(t *testing.T) {
	tests := []struct {
		input    string
		contains string // The cleaned path should contain this substring
	}{
		{
			input:    "some//path.txt",
			contains: "some",
		},
		{
			input:    "some/./path.txt",
			contains: "path.txt",
		},
		{
			input:    "some/other/../path.txt",
			contains: "path.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ValidatePath(tt.input)
			if err != nil {
				t.Fatalf("ValidatePath(%q) error: %v", tt.input, err)
			}

			// The path should not contain // or /./ after cleaning
			if filepath.Clean(result) != result {
				t.Errorf("ValidatePath(%q) returned uncleaned path: %q", tt.input, result)
			}
		})
	}
}

// An absolute path that resolves inside root must be accepted. It was rejected
// outright, which refused a project's own `/…/repo/.coverctl.yaml` while the
// relative form worked — and contradicted the remediation text callers get,
// which says absolute paths under the project root are accepted.
func TestValidateScopedPath_AcceptsAbsoluteInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "coverctl.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	abs := filepath.Join(root, "coverctl.yaml")
	got, err := ValidateScopedPath(abs, root)
	if err != nil {
		t.Fatalf("ValidateScopedPath(%q, %q) = %v, want it accepted", abs, root, err)
	}

	rel, relErr := ValidateScopedPath("coverctl.yaml", root)
	if relErr != nil {
		t.Fatalf("relative form failed: %v", relErr)
	}
	if got != rel {
		t.Errorf("absolute and relative forms disagree: %q vs %q", got, rel)
	}
}

// The containment check, not the absolute/relative distinction, is what keeps
// scope. An absolute path outside root must still be refused.
func TestValidateScopedPath_StillRejectsAbsoluteOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	for _, p := range []string{target, "/etc/passwd", filepath.Join(outside, "does-not-exist")} {
		if _, err := ValidateScopedPath(p, root); err != ErrPathEscapesBase {
			t.Errorf("ValidateScopedPath(%q) = %v, want ErrPathEscapesBase", p, err)
		}
	}
}

// A file that does not exist yet, under a root reached through a symlink, must
// still be recognized as inside root — the case that motivated resolving the
// deepest existing ancestor rather than the whole path.
func TestValidateScopedPath_NonexistentUnderSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	for _, p := range []string{"new-file.out", "sub/dir/new-file.out"} {
		if _, err := ValidateScopedPath(p, link); err != nil {
			t.Errorf("ValidateScopedPath(%q, symlinked root) = %v, want accepted", p, err)
		}
		if _, err := ValidateScopedPath(filepath.Join(link, p), link); err != nil {
			t.Errorf("absolute %q under symlinked root = %v, want accepted", p, err)
		}
	}
}
