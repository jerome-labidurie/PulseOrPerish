// Package fscrypt provides encryption and decryption functionality for files.
// Code is heavily inspired by David Jiang Medium article
// https://medium.com/@djiangtaz/tar-gzip-and-encrypt-files-in-golang-by-chaining-io-writer-s-737a6cc40894

package fscrypt

import (
	"archive/tar"
	"compress/gzip"
	"compress/lzw"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	nacl "github.com/nathants/go-libsodium"
)

type FsCrypt struct {
	Compress string // compression algo
	Password string // pwd for [en|de]crypt
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

	log.Debug().Str("fname", filename).Msg("Add file to tar")
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = filename
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

// encryptFiles creates an encrypted and compressed archive from a list of files.
// It uses goroutines to handle the tar/zip compression stream and libsodium encryption simultaneously.
func (fc FsCrypt) EncryptFiles(filesin []string, fileout string) error {
	var xzw io.WriteCloser
	writer, err := os.Create(fileout)
	if err != nil {
		return err
	}
	defer writer.Close()

	preader, pwriter := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if fc.Compress == "lzw" {
			xzw = lzw.NewWriter(pwriter, lzw.LSB, 8)
		} else {
			xzw = gzip.NewWriter(pwriter)
		}
		tw := tar.NewWriter(xzw)
		for _, filename := range filesin {
			err := fc.addToArchive(tw, filename)
			if err != nil {
				log.Error().Err(err)
				continue
			}
		}
		// closing the Writers must follow the data flow, starting from the source
		// if using the defer call, make sure follow the FILO rule
		tw.Close()
		xzw.Close()
		// pwriter has to be closed in a goroutine, or the program will lock
		pwriter.Close()
	}()

	key := sha256.Sum256([]byte(fc.Password))
	nacl.Init()
	go func() {
		defer wg.Done()
		err := nacl.StreamEncrypt(key[:], preader, writer)
		if err != nil {
			log.Error().Err(err)
		}
	}()
	wg.Wait()
	return nil
}

// decryptFile decrypts a .nacl file back into an uncyphered tar stream.
func (fc FsCrypt) DecryptFile(filein string, fileout string) error {
	reader, err := os.Open(filein)
	if err != nil {
		return err
	}
	defer reader.Close()

	// TODO: check fileout does not exists before trunc it !!
	writer, err := os.Create(fileout)
	if err != nil {
		return err
	}
	defer writer.Close()

	log.Debug().Str("in", filein).Str("out", fileout).Msg("Decrypt")

	key := sha256.Sum256([]byte(fc.Password))
	nacl.Init()
	err = nacl.StreamDecrypt(key[:], reader, writer)
	if err != nil {
		return err
	}
	return nil
}

func (fc FsCrypt) GetCryptedFileName(idx int) string {
	return fmt.Sprintf("file_%04d.tar.%s.nacl", idx, fc.Compress)
}

func (fc FsCrypt) GetPlainFileName(fname string) string {
	return strings.Trim(fname, ".nacl")
}
