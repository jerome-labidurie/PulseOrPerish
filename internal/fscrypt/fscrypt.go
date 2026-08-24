// Package fscrypt provides encryption and decryption functionality for files.
// Code is heavily inspired by David Jiang Medium article
// https://medium.com/@djiangtaz/tar-gzip-and-encrypt-files-in-golang-by-chaining-io-writer-s-737a6cc40894

package fscrypt

import (
	"archive/tar"
	"compress/gzip"
	"compress/lzw"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/argon2"

	"github.com/nathants/go-libsodium"
)

var ErrNoFilesToEncrypt = errors.New("no files to encrypt")

const (
	FileExtension string = "pop" // encrypted file extension
	saltSize             = 16    // argon2 salt size in bytes
	argon2Time    uint32 = 3
	argon2Memory  uint32 = 64 * 1024 // 64 MiB
	argon2Threads uint8  = 4
	keySize       uint32 = 32
)

type FsCrypt struct {
	Compress string // compression algo
	Password []byte // pwd for [en|de]crypt
	Logger   zerolog.Logger
}

// Init initializes the libsodium library.
// It must be called before any other libsodium functions.
func (fc FsCrypt) Init() {
	libsodium.Init()
}

// Clear zeroes the password bytes in memory.
// Call it (e.g. via defer) once encryption/decryption is done.
func (fc *FsCrypt) Clear() {
	clear(fc.Password)
}

// KDF: pwdToKey derives a key from the password and salt using Argon2id.
func (fc FsCrypt) pwdToKey(salt []byte) []byte {
	return argon2.IDKey(fc.Password, salt, argon2Time, argon2Memory, argon2Threads, keySize)
}

// addToArchive adds a file to the given tar.Writer.
// It preserves permissions but uses the provided filename in the archive header.
func (fc FsCrypt) addToArchive(tw *tar.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	fc.Logger.Debug().Str("fname", filename).Msg("Add file to tar")
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = strings.TrimLeft(filename, "./")
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}
	return nil
}

// EncryptPaths creates an encrypted and compressed archive from a list of input
// paths. Each input path may be a regular file or a directory, in which case
// regular files are collected recursively.
func (fc FsCrypt) EncryptPaths(filesin []string, fileout string) error {
	regularFiles, err := collectRegularFiles(filesin)
	if err != nil {
		return err
	}
	// Validate input
	if len(regularFiles) == 0 {
		return ErrNoFilesToEncrypt
	}

	fc.Logger.Debug().Strs("files", regularFiles).Str("out", fileout).Msg("Encrypting files")

	var xzw io.WriteCloser
	writer, err := os.Create(fileout) // erase fileout if exists
	if err != nil {
		return err
	}
	defer writer.Close()

	// Generate salt and derive key
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}
	// write salt at the beginning of the file
	if _, err := writer.Write(salt); err != nil {
		return fmt.Errorf("failed to write salt: %w", err)
	}
	fc.Logger.Debug().Str("salt", fmt.Sprintf("%x", salt)).Msg("Generated salt for encryption")
	key := fc.pwdToKey(salt)

	preader, pwriter := io.Pipe()
	var (
		wg         sync.WaitGroup
		tarErr     error
		encryptErr error
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		if fc.Compress == "lzw" {
			xzw = lzw.NewWriter(pwriter, lzw.LSB, 8)
		} else {
			xzw = gzip.NewWriter(pwriter)
		}
		tw := tar.NewWriter(xzw)
		for _, filename := range regularFiles {
			if err := fc.addToArchive(tw, filename); err != nil {
				fc.Logger.Error().Err(err).Str("file", filename).Msg("failed to add file to archive")
				continue
			}
		}
		// closing the Writers must follow the data flow, starting from the source
		// if using the defer call, make sure follow the FILO rule
		if err := tw.Close(); err != nil {
			tarErr = fmt.Errorf("failed to close tar writer: %w", err)
			pwriter.CloseWithError(tarErr)
			return
		}
		if err := xzw.Close(); err != nil {
			tarErr = fmt.Errorf("failed to close compressor: %w", err)
			pwriter.CloseWithError(tarErr)
			return
		}
		// pwriter has to be closed in a goroutine, or the program will lock
		pwriter.Close()
	}()

	go func() {
		defer wg.Done()
		if err := libsodium.StreamEncrypt(key, preader, writer); err != nil {
			encryptErr = err
			fc.Logger.Error().Err(err).Msg("stream encryption failed")
		}
	}()

	wg.Wait()
	if err := errors.Join(tarErr, encryptErr); err != nil {
		return err
	}
	fc.Logger.Info().Str("fname", fileout).Msg("Encrypted archive")
	return nil
}

func collectRegularFiles(roots []string) ([]string, error) {
	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk input %q: %w", root, err)
		}
	}
	return files, nil
}

// decryptFile decrypts a encrypted file back into an uncyphered tar stream.
func (fc FsCrypt) DecryptFile(filein string, fileout string) error {
	reader, err := os.Open(filein)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer, err := os.OpenFile(fileout, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666) // do NOT erase fileout
	if err != nil {
		return err
	}
	defer writer.Close()

	fc.Logger.Debug().Str("in", filein).Str("out", fileout).Msg("Decrypting archive")

	// read salt from the beginning of the file
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(reader, salt); err != nil {
		return fmt.Errorf("failed to read salt: %w", err)
	}
	fc.Logger.Debug().Str("salt", fmt.Sprintf("%x", salt)).Msg("Read salt for decryption")
	key := fc.pwdToKey(salt)

	// decrypt rest of the file
	err = libsodium.StreamDecrypt(key, reader, writer)
	if err != nil {
		return err
	}
	fc.Logger.Info().Str("out", fileout).Str("in", filein).Msg("Decrypted archive")
	return nil
}

func (fc FsCrypt) GetCryptedFileName(idx int) string {
	return fmt.Sprintf("file_%04d.tar.%s.%s", idx, fc.Compress, FileExtension)
}

func (fc FsCrypt) GetPlainFileName(fname string) string {
	return strings.TrimSuffix(fname, "."+FileExtension)
}
