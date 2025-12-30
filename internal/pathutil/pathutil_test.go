package pathutil

import (
	"runtime"
	"testing"
)

// === Normalize Tests ===

func TestNormalize_UnixPath(t *testing.T) {
	result := Normalize("/home/user/project")
	if runtime.GOOS == "windows" {
		// On Windows, filepath.Clean converts forward slashes to backslashes
		expected := `\home\user\project`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		expected := "/home/user/project"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

func TestNormalize_WindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	result := Normalize(`C:\Users\dev\project`)
	expected := `C:\Users\dev\project`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNormalize_MixedSeparators(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	result := Normalize(`C:/Users/dev\project`)
	expected := `C:\Users\dev\project`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNormalize_PathWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	result := Normalize(`C:\Program Files\My App\project`)
	expected := `C:\Program Files\My App\project`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNormalize_RedundantSeparators(t *testing.T) {
	result := Normalize("path//to///file")
	if runtime.GOOS == "windows" {
		expected := `path\to\file`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		expected := "path/to/file"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

func TestNormalize_TrailingSlash(t *testing.T) {
	result := Normalize("path/to/dir/")
	if runtime.GOOS == "windows" {
		expected := `path\to\dir`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		expected := "path/to/dir"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

// === IsAbsolute Tests ===

func TestIsAbsolute_WindowsDriveLetter(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	if !IsAbsolute(`C:\Users\dev`) {
		t.Error("C:\\Users\\dev should be absolute")
	}
	if !IsAbsolute(`D:\`) {
		t.Error("D:\\ should be absolute")
	}
}

func TestIsAbsolute_UnixPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test")
	}
	if !IsAbsolute("/home/user") {
		t.Error("/home/user should be absolute")
	}
}

func TestIsAbsolute_RelativePath(t *testing.T) {
	if IsAbsolute("relative/path") {
		t.Error("relative/path should not be absolute")
	}
	if IsAbsolute("./local") {
		t.Error("./local should not be absolute")
	}
}

// === IsUNC Tests ===

func TestIsUNC_ValidUNCPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On non-Windows, UNC detection should return false
		if IsUNC(`\\server\share\folder`) {
			t.Error("UNC detection should return false on non-Windows")
		}
		return
	}
	if !IsUNC(`\\server\share\folder`) {
		t.Error("\\\\server\\share\\folder should be UNC")
	}
}

func TestIsUNC_NotUNCPath(t *testing.T) {
	if IsUNC(`C:\Users\dev`) {
		t.Error("C:\\Users\\dev should not be UNC")
	}
	if IsUNC("/home/user") {
		t.Error("/home/user should not be UNC")
	}
}

// === Conversion Tests ===

func TestToSlash(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, ToSlash is a no-op for Unix paths
		result := ToSlash("/home/user/project")
		expected := "/home/user/project"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
		return
	}
	result := ToSlash(`C:\Users\dev\project`)
	expected := "C:/Users/dev/project"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFromSlash(t *testing.T) {
	result := FromSlash("path/to/file")
	if runtime.GOOS == "windows" {
		expected := `path\to\file`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		expected := "path/to/file"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

// === Join Tests ===

func TestJoin_Basic(t *testing.T) {
	result := Join("path", "to", "file")
	if runtime.GOOS == "windows" {
		expected := `path\to\file`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		expected := "path/to/file"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

func TestJoin_WithRootDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	result := Join(`C:\Users`, "dev", "project")
	expected := `C:\Users\dev\project`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// === Rel Tests ===

func TestRel_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		result, err := Rel(`C:\Users\dev`, `C:\Users\dev\project\file.go`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := `project\file.go`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		result, err := Rel("/home/user", "/home/user/project/file.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "project/file.go"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

// === Dir Tests ===

func TestDir_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		result := Dir(`C:\Users\dev\file.go`)
		expected := `C:\Users\dev`
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		result := Dir("/home/user/file.go")
		expected := "/home/user"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

// === Base Tests ===

func TestBase_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		result := Base(`C:\Users\dev\file.go`)
		expected := "file.go"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	} else {
		result := Base("/home/user/file.go")
		expected := "file.go"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

// === Ext Tests ===

func TestExt_Basic(t *testing.T) {
	result := Ext("file.go")
	expected := ".go"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExt_NoExtension(t *testing.T) {
	result := Ext("Makefile")
	expected := ""
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
