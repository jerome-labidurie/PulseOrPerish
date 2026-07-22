package fscrypt

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"compress/lzw"
	"io"
	"os"
	"path/filepath"
	"testing"

	"pulseorperish/internal/testkit/fshelpers"

	"github.com/rs/zerolog"
)

func testFsCrypt(password []byte, compress string) FsCrypt {
	return FsCrypt{Password: password, Compress: compress, Logger: zerolog.Nop()}
}

func TestPwdToKey_Length(t *testing.T) {
	fc := testFsCrypt([]byte("password-1234"), "")
	salt := make([]byte, saltSize)
	key := fc.pwdToKey(salt)
	if len(key) != int(keySize) {
		t.Errorf("expected key length %d, got %d", keySize, len(key))
	}
}

func TestPwdToKey_Deterministic(t *testing.T) {
	fc := testFsCrypt([]byte("same-password"), "")
	salt := []byte("0123456789abcdef") // 16 bytes

	key1 := fc.pwdToKey(salt)
	key2 := fc.pwdToKey(salt)
	if !bytes.Equal(key1, key2) {
		t.Error("same password+salt should always produce the same key")
	}
}

func TestPwdToKey_DifferentSalts(t *testing.T) {
	fc := testFsCrypt([]byte("same-password"), "")
	salt1 := bytes.Repeat([]byte{0x00}, saltSize)
	salt2 := bytes.Repeat([]byte{0xff}, saltSize)

	key1 := fc.pwdToKey(salt1)
	key2 := fc.pwdToKey(salt2)
	if bytes.Equal(key1, key2) {
		t.Error("different salts should produce different keys")
	}
}

func TestPwdToKey_DifferentPasswords(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, saltSize)

	key1 := testFsCrypt([]byte("password-a"), "").pwdToKey(salt)
	key2 := testFsCrypt([]byte("password-b"), "").pwdToKey(salt)
	if bytes.Equal(key1, key2) {
		t.Error("different passwords should produce different keys")
	}
}

func TestPwdToKey_EmptyPassword(t *testing.T) {
	fc := testFsCrypt([]byte(""), "")
	salt := bytes.Repeat([]byte{0x01}, saltSize)
	key := fc.pwdToKey(salt)
	if len(key) != int(keySize) {
		t.Errorf("expected key length %d, got %d", keySize, len(key))
	}
}

// Clear
func TestClear(t *testing.T) {
	pwd := []byte("password1234")
	fc := testFsCrypt(pwd, "")
	fc.Clear()
	for i, b := range fc.Password {
		if b != 0 {
			t.Errorf("byte %d not zeroed after Clear(): got 0x%02x", i, b)
		}
	}
}

// Test GetCryptedFileName
func TestGetCryptedFileName(t *testing.T) {
	tests := []struct {
		compress string
		idx      int
		want     string
	}{
		{"gz", 0, "file_0000.tar.gz." + FileExtension},
		{"gz", 42, "file_0042.tar.gz." + FileExtension},
		{"lzw", 1, "file_0001.tar.lzw." + FileExtension},
		{"gz", 6969, "file_6969.tar.gz." + FileExtension},
	}
	for _, tc := range tests {
		fc := testFsCrypt(nil, tc.compress)
		got := fc.GetCryptedFileName(tc.idx)
		if got != tc.want {
			t.Errorf("GetCryptedFileName(%d) with %s = %q, want %q", tc.idx, tc.compress, got, tc.want)
		}
	}
}

// Test GetPlainFileName
func TestGetPlainFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"file_0000.tar.gz." + FileExtension, "file_0000.tar.gz"},
		{"file_0001.tar.lzw." + FileExtension, "file_0001.tar.lzw"},
		{"file." + FileExtension, "file"},
		{"file_no_extension", "file_no_extension"}, // no .pop suffix — unchanged
	}
	fc := testFsCrypt(nil, "")
	for _, tc := range tests {
		got := fc.GetPlainFileName(tc.input)
		if got != tc.want {
			t.Errorf("GetPlainFileName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Error cases ---

func TestEncryptFiles_NonExistentInput(t *testing.T) {
	tmpDir := t.TempDir()
	fc := testFsCrypt([]byte("pass"), "gz")
	fc.Init()

	// A non-existent file is logged and skipped; the archive is still created.
	outFile := filepath.Join(tmpDir, fc.GetCryptedFileName(0))
	err := fc.EncryptFiles([]string{"/nonexistent/file.txt"}, outFile)
	if err != nil {
		t.Errorf("expected nil error (non-existent file is skipped), got %v", err)
	}
	fshelpers.AssertFilesExist(t, []string{outFile})
}

func TestEncryptFiles_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	fc := testFsCrypt([]byte("pass"), "gz")
	fc.Init()

	outFile := filepath.Join(tmpDir, fc.GetCryptedFileName(0))
	err := fc.EncryptFiles([]string{}, outFile)
	if err == nil {
		t.Error("expected error for empty file list, got nil")
	}
}

func TestDecryptFile_NonExistentInput(t *testing.T) {
	tmpDir := t.TempDir()
	fc := testFsCrypt([]byte("pass"), "gz")
	fc.Init()

	err := fc.DecryptFile("/nonexistent/file."+FileExtension, filepath.Join(tmpDir, "out.tar.gz"))
	if err == nil {
		t.Error("expected error for non-existent input file, got nil")
	}
}

func TestDecryptFile_ExistingOutput(t *testing.T) {
	tmpDir := t.TempDir()
	fc := testFsCrypt([]byte("pass"), "gz")
	fc.Init()

	// Create a dummy input file (just needs to exist for the open call).
	inFile := fshelpers.CreateTestFile(t, tmpDir, "dummy."+FileExtension)

	// Pre-create the output file — DecryptFile uses O_EXCL and should refuse.
	outFile := fshelpers.CreateTestFile(t, tmpDir, "already_exists.tar.gz")

	err := fc.DecryptFile(inFile, outFile)
	if err == nil {
		t.Error("expected error when output file already exists (O_EXCL), got nil")
	}
}

func TestDecryptFile_TruncatedFile(t *testing.T) {
	tmpDir := t.TempDir()
	fc := testFsCrypt([]byte("pass"), "gz")
	fc.Init()

	// Write fewer bytes than saltSize — ReadFull should fail.
	inFile := filepath.Join(tmpDir, "truncated."+FileExtension)
	os.WriteFile(inFile, []byte{0x01, 0x02}, 0644)

	outFile := filepath.Join(tmpDir, "out.tar.gz")
	err := fc.DecryptFile(inFile, outFile)
	if err == nil {
		t.Error("expected error reading salt from truncated file, got nil")
	}
}

// --- Round-trip tests (slow: ~300ms each due to argon2id) ---

// encryptDecryptRoundTrip is a helper that:
//  1. Creates a temp file with the given content
//  2. Encrypts it with the given compressor
//  3. Decrypts the archive back
//  4. Returns the decrypted file path and a cleanup function
func encryptDecryptRoundTrip(t *testing.T, compress, content string) (decryptedPath string) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	fc := testFsCrypt([]byte("password1234"), compress)
	fc.Init()

	// Encrypt
	encFile := filepath.Join(tmpDir, fc.GetCryptedFileName(0))
	if err := fc.EncryptFiles([]string{srcFile}, encFile); err != nil {
		t.Fatalf("EncryptFiles failed: %v", err)
	}
	fshelpers.AssertFilesExist(t, []string{encFile})

	// Decrypt
	decFile := filepath.Join(tmpDir, fc.GetPlainFileName(fc.GetCryptedFileName(0)))
	if err := fc.DecryptFile(encFile, decFile); err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}
	fshelpers.AssertFilesExist(t, []string{decFile})

	return decFile
}

// readFirstTarEntry opens an encrypted archive, decompresses it using the
// provided decompress function, and returns the name and content of the first
// tar entry. The decompress function must return an io.ReadCloser wrapping the
// given io.Reader (e.g. gzip.NewReader or lzw.NewReader).
func readFirstTarEntry(t *testing.T, archivePath string, decompress func(io.Reader) (io.ReadCloser, error)) (name, content string) {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open decrypted archive: %v", err)
	}
	defer f.Close()

	dc, err := decompress(f)
	if err != nil {
		t.Fatalf("failed to create decompressor: %v", err)
	}
	defer dc.Close()

	tr := tar.NewReader(dc)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("failed to read tar header: %v", err)
	}
	data, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("failed to read tar entry content: %v", err)
	}
	return hdr.Name, string(data)
}

// TestEncryptDecrypt_RoundTrip verifies the full encryption/decryption pipeline
// for each supported compressor (gz, lzw). It encrypts a source file, decrypts
// the resulting archive, decompresses the tar stream, and checks that the
// original file content and name are preserved end-to-end.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		compress   string
		decompress func(io.Reader) (io.ReadCloser, error)
	}{
		{
			compress: "gz",
			decompress: func(r io.Reader) (io.ReadCloser, error) {
				return gzip.NewReader(r)
			},
		},
		{
			compress: "lzw",
			decompress: func(r io.Reader) (io.ReadCloser, error) {
				return lzw.NewReader(r, lzw.LSB, 8), nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.compress, func(t *testing.T) {
			wantContent := "hello, " + tc.compress + " world!"
			decFile := encryptDecryptRoundTrip(t, tc.compress, wantContent)

			name, gotContent := readFirstTarEntry(t, decFile, tc.decompress)
			if gotContent != wantContent {
				t.Errorf("content mismatch: got %q, want %q", gotContent, wantContent)
			}
			if filepath.Base(name) != "source.txt" {
				t.Errorf("unexpected tar entry name: %q", name)
			}
		})
	}
}

func TestEncryptDecrypt_WrongPassword(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := fshelpers.CreateTestFile(t, tmpDir, "source.txt")

	fcEnc := testFsCrypt([]byte("correct-password"), "gz")
	fcEnc.Init()

	encFile := filepath.Join(tmpDir, fcEnc.GetCryptedFileName(0))
	if err := fcEnc.EncryptFiles([]string{srcFile}, encFile); err != nil {
		t.Fatalf("EncryptFiles failed: %v", err)
	}

	fcDec := testFsCrypt([]byte("wrong-password"), "gz")
	fcDec.Init()

	decFile := filepath.Join(tmpDir, fcEnc.GetPlainFileName(fcEnc.GetCryptedFileName(0)))
	err := fcDec.DecryptFile(encFile, decFile)
	if err == nil {
		t.Error("expected decryption to fail with wrong password, got nil error")
	}
}
