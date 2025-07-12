package virtualfs

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestZipFileSystemCompliance(t *testing.T) {
	// Create a temporary directory for the zip file
	tempDir := t.TempDir()
	zipFilePath := filepath.Join(tempDir, "test.zip")
	createTestZipFile(zipFilePath, t)
	defer os.Remove(zipFilePath)

	zfs, err := NewZipFileSystem(zipFilePath)
	if err != nil {
		t.Fatalf("Failed to create ZipFileSystem: %v", err)
	}
	defer zfs.(*ZipFileSystem).Close()

	// Test Stat compliance
	fileInfo, err := zfs.Stat("testfile.txt")
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if fileInfo.Name() != "testfile.txt" {
		t.Errorf("Expected file name 'testfile.txt', got '%s'", fileInfo.Name())
	}

	// Test Open compliance
	file, err := zfs.Open("testfile.txt")
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Errorf("Read failed: %v", err)
	}
	if n == 0 {
		t.Errorf("Expected to read data, but got 0 bytes")
	}

	// Test ReadDir compliance
	dirEntries, err := zfs.(*ZipFileSystem).ReadDir("testdir/")
	if err != nil {
		t.Errorf("ReadDir failed: %v", err)
	}
	if len(dirEntries) != 3 || dirEntries[0].Name() != "nestedfile.txt" || dirEntries[1].Name() != "subdir1/" || dirEntries[2].Name() != "subdir2/" {
		t.Errorf("Expected directory entries 'nestedfile.txt', 'subdir1/', 'subdir2/', got '%v'", dirEntries)
	}

	dirEntries, err = zfs.(*ZipFileSystem).ReadDir("testdir/subdir1/")
	if err != nil {
		t.Errorf("ReadDir failed: %v", err)
	}
	if len(dirEntries) != 1 || dirEntries[0].Name() != "file1.txt" {
		t.Errorf("Expected directory entry 'file1.txt', got '%v'", dirEntries)
	}

	dirEntries, err = zfs.(*ZipFileSystem).ReadDir("testdir/subdir2/")
	if err != nil {
		t.Errorf("ReadDir failed: %v", err)
	}
	if len(dirEntries) != 1 || dirEntries[0].Name() != "file2.txt" {
		t.Errorf("Expected directory entry 'file2.txt', got '%v'", dirEntries)
	}
}

func createTestZipFile(zipFilePath string, t *testing.T) {
	file, err := os.Create(zipFilePath)
	if err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Add a test file
	w, err := zipWriter.Create("testfile.txt")
	if err != nil {
		t.Fatalf("Failed to add test file to zip: %v", err)
	}
	_, err = w.Write([]byte("This is a test file."))
	if err != nil {
		t.Fatalf("Failed to write to test file in zip: %v", err)
	}

	// Add a nested directory and file
	w, err = zipWriter.Create("testdir/nestedfile.txt")
	if err != nil {
		t.Fatalf("Failed to add nested file to zip: %v", err)
	}
	_, err = w.Write([]byte("This is a nested test file."))
	if err != nil {
		t.Fatalf("Failed to write to nested file in zip: %v", err)
	}

	// Add additional subdirectories and files
	w, err = zipWriter.Create("testdir/subdir1/file1.txt")
	if err != nil {
		t.Fatalf("Failed to add file1.txt to zip: %v", err)
	}
	_, err = w.Write([]byte("This is file1 in subdir1."))
	if err != nil {
		t.Fatalf("Failed to write to file1.txt in zip: %v", err)
	}

	w, err = zipWriter.Create("testdir/subdir2/file2.txt")
	if err != nil {
		t.Fatalf("Failed to add file2.txt to zip: %v", err)
	}
	_, err = w.Write([]byte("This is file2 in subdir2."))
	if err != nil {
		t.Fatalf("Failed to write to file2.txt in zip: %v", err)
	}
}

func TestMultipleZipFilesWithMergedContent(t *testing.T) {
	// Create a temporary directory for the zip files
	tempDir := t.TempDir()
	zipFiles := []string{
		filepath.Join(tempDir, "testzip1.zip"),
		filepath.Join(tempDir, "testzip2.zip"),
		filepath.Join(tempDir, "testzip3.zip"),
	}

	// Define test cases for zip files
	testCases := []struct {
		zipFilePath string
		content     map[string]string
	}{
		{
			zipFiles[0],
			map[string]string{
				"testdir/subdir1/file1.txt": "Content of file1 in zip1",
				"testdir/subdir2/file2.txt": "Content of file2 in zip1",
			},
		},
		{
			zipFiles[1],
			map[string]string{
				"testdir/subdir1/file3.txt": "Content of file3 in zip2",
				"testdir/subdir2/file4.txt": "Content of file4 in zip2",
			},
		},
		{
			zipFiles[2],
			map[string]string{
				"testdir/subdir1/file5.txt": "Content of file5 in zip3",
				"testdir/subdir2/file6.txt": "Content of file6 in zip3",
			},
		},
	}

	// Create test zip files
	for _, tc := range testCases {
		createTestZipFileWithContent(tc.zipFilePath, tc.content, t)
	}

	// Test combining multiple zip files
	zfs, err := NewZipFileSystem(filepath.Join(tempDir, "testzip*.zip"))
	if err != nil {
		t.Fatalf("Failed to create ZipFileSystem: %v", err)
	}
	defer zfs.(*ZipFileSystem).Close()

	// Define expected results for subdir1 and subdir2
	expectedResults := map[string][]struct {
		name    string
		content string
	}{
		"testdir/subdir1/": {
			{"file1.txt", "Content of file1 in zip1"},
			{"file3.txt", "Content of file3 in zip2"},
			{"file5.txt", "Content of file5 in zip3"},
		},
		"testdir/subdir2/": {
			{"file2.txt", "Content of file2 in zip1"},
			{"file4.txt", "Content of file4 in zip2"},
			{"file6.txt", "Content of file6 in zip3"},
		},
	}

	// Test ReadDir and Open for each subdir
	for subdir, expectedFiles := range expectedResults {
		dirEntries, err := zfs.(*ZipFileSystem).ReadDir(subdir)
		if err != nil {
			t.Errorf("ReadDir failed for %s: %v", subdir, err)
		}
		if len(dirEntries) != len(expectedFiles) {
			t.Errorf("Expected %d entries in %s, got %d", len(expectedFiles), subdir, len(dirEntries))
		}

		for i, expectedFile := range expectedFiles {
			if dirEntries[i].Name() != expectedFile.name {
				t.Errorf("Expected entry '%s' in %s, got '%s'", expectedFile.name, subdir, dirEntries[i].Name())
			}

			file, err := zfs.Open(subdir + expectedFile.name)
			if err != nil {
				t.Errorf("Failed to open file %s: %v", expectedFile.name, err)
				continue
			}
			defer file.Close()

			buf := make([]byte, 1024)
			n, err := file.Read(buf)
			if err != nil && err.Error() != "EOF" {
				t.Errorf("Failed to read file %s: %v", expectedFile.name, err)
			}
			if string(buf[:n]) != expectedFile.content {
				t.Errorf("Content mismatch for file %s: expected '%s', got '%s'", expectedFile.name, expectedFile.content, string(buf[:n]))
			}
		}
	}

	// Test non-existent file in an existing folder
	_, err = zfs.Open("testdir/subdir1/nonexistent.txt")
	if err == nil {
		t.Errorf("Expected error when opening non-existent file in existing folder, got nil")
	}

	// Test non-existent folder
	_, err = zfs.(*ZipFileSystem).ReadDir("testdir/nonexistentfolder/")
	if err == nil {
		t.Errorf("Expected error when reading non-existent folder, got nil")
	}

	// Test non-existent file in a non-existent folder
	_, err = zfs.Open("testdir/nonexistentfolder/nonexistent.txt")
	if err == nil {
		t.Errorf("Expected error when reading non-existent folder, got nil")
	}
}

// Corner case: Test empty zip file
func TestEmptyZip(t *testing.T) {
	tempDir := t.TempDir()

	emptyZipFile := filepath.Join(tempDir, "empty.zip")
	createTestZipFileWithContent(emptyZipFile, map[string]string{}, t)

	zfsEmpty, err := NewZipFileSystem(emptyZipFile)
	if err != nil {
		t.Fatalf("Failed to create ZipFileSystem for empty zip: %v", err)
	}
	defer zfsEmpty.(*ZipFileSystem).Close()

	dirEntries, err := zfsEmpty.(*ZipFileSystem).ReadDir(".")
	if err == nil {
		t.Errorf("ReadDir didn't fail for empty zip")
	}
	if len(dirEntries) != 0 {
		t.Errorf("Expected 0 entries in empty zip, got %d", len(dirEntries))
	}
}

func createTestZipFileWithContent(zipFilePath string, content map[string]string, t *testing.T) {
	file, err := os.Create(zipFilePath)
	if err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for path, data := range content {
		w, err := zipWriter.Create(path)
		if err != nil {
			t.Fatalf("Failed to add file %s to zip: %v", path, err)
		}
		_, err = w.Write([]byte(data))
		if err != nil {
			t.Fatalf("Failed to write to file %s in zip: %v", path, err)
		}
	}
}

func TestMultipleZipFilesWithWalkDir(t *testing.T) {
	// Create a temporary directory for the zip files
	tempDir := t.TempDir()
	zipFiles := []string{
		filepath.Join(tempDir, "testzip1.zip"),
		filepath.Join(tempDir, "testzip2.zip"),
		filepath.Join(tempDir, "testzip3.zip"),
	}

	// Define test cases for zip files
	testCases := []struct {
		zipFilePath string
		content     map[string]string
	}{
		{
			zipFiles[0],
			map[string]string{
				"testdir/subdir1/file1.txt": "Content of file1 in zip1",
				"testdir/subdir2/file2.txt": "Content of file2 in zip1",
			},
		},
		{
			zipFiles[1],
			map[string]string{
				"testdir/subdir1/file3.txt": "Content of file3 in zip2",
				"testdir/subdir2/file4.txt": "Content of file4 in zip2",
			},
		},
		{
			zipFiles[2],
			map[string]string{
				"testdir/subdir1/file5.txt": "Content of file5 in zip3",
				"testdir/subdir2/file6.txt": "Content of file6 in zip3",
			},
		},
	}

	// Create test zip files
	for _, tc := range testCases {
		createTestZipFileWithContent(tc.zipFilePath, tc.content, t)
	}

	// Test combining multiple zip files
	zfs, err := NewZipFileSystem(filepath.Join(tempDir, "testzip*.zip"))
	if err != nil {
		t.Fatalf("Failed to create ZipFileSystem: %v", err)
	}
	defer zfs.(*ZipFileSystem).Close()

	// Define expected results
	expectedFiles := map[string]string{
		"testdir/subdir1/file1.txt": "Content of file1 in zip1",
		"testdir/subdir1/file3.txt": "Content of file3 in zip2",
		"testdir/subdir1/file5.txt": "Content of file5 in zip3",
		"testdir/subdir2/file2.txt": "Content of file2 in zip1",
		"testdir/subdir2/file4.txt": "Content of file4 in zip2",
		"testdir/subdir2/file6.txt": "Content of file6 in zip3",
	}

	// Use fs.WalkDir to traverse the merged content
	walkedFiles := make(map[string]string)
	err = fs.WalkDir(zfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			file, err := zfs.Open(path)
			if err != nil {
				t.Errorf("Failed to open file %s: %v", path, err)
				return nil
			}
			defer file.Close()

			buf := make([]byte, 1024)
			n, err := file.Read(buf)
			if err != nil && err.Error() != "EOF" {
				t.Errorf("Failed to read file %s: %v", path, err)
				return nil
			}
			walkedFiles[path] = string(buf[:n])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	// Verify walked files against expected results
	if len(walkedFiles) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(walkedFiles))
	}
	for path, content := range expectedFiles {
		if walkedFiles[path] != content {
			t.Errorf("Content mismatch for file %s: expected '%s', got '%s'", path, content, walkedFiles[path])
		}
	}
}
