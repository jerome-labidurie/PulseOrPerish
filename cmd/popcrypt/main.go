package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"pulseorperish/internal/fscrypt"
)

// Version, BuildDate, and CommitHash are set during build via ldflags.
var (
	Version    = "dev"
	BuildDate  = "unknown"
	CommitHash = "unknown"
)

// WalkDirectory recursively traverses a directory and returns the list of regular files.
func WalkDirectory(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "" {
			return err
		}
		if !d.Type().IsRegular() {
			return nil // Skip directories and symlinks etc.
		}
		files = append(files, "./"+path)
		return nil
	})
	if err != nil {
		return files, fmt.Errorf("error walking directory: %w", err)
	}

	return files, nil
}

func main() {
	var decrypt bool
	var encrypt bool

	help := flag.Bool("help", false, "Display help")
	password := flag.String("password", "", "Password to encrypt/decrypt")
	flag.BoolVar(&decrypt, "decrypt", false, "Decrypt a file")
	flag.BoolVar(&decrypt, "d", false, "Alias for -decrypt")
	flag.BoolVar(&encrypt, "encrypt", false, "Encrypt files")
	flag.BoolVar(&encrypt, "e", false, "Alias for -encrypt")
	compressor := flag.String("comp", "gz", "Compression to use gz|lzw")
	version := flag.Bool("version", false, "Display version")

	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *version {
		fmt.Printf("PoPCrypt version %s (built %s, commit %s)\n", Version, BuildDate, CommitHash)
		os.Exit(0)
	}

	if *password == "" {
		flag.PrintDefaults()
		log.Fatalf("password is required")
	}
	if encrypt && decrypt {
		flag.PrintDefaults()
		log.Fatalf("Cannot encrypt and decrypt")
	}
	if encrypt && len(flag.Args()) < 1 {
		flag.PrintDefaults()
		log.Fatalf("-encrypt needs files")
	}
	if decrypt && len(flag.Args()) != 1 {
		flag.PrintDefaults()
		log.Fatalf("-decrypt needs one file only")
	}
	if *compressor != "gz" && *compressor != "lzw" {
		flag.PrintDefaults()
		log.Fatalf("Only gz & lzw supported, got %v", *compressor)
	}
	// log.Printf("%v\n", flag.Args())

	fc := fscrypt.FsCrypt{
		Password: *password,
		Compress: *compressor,
	}

	if encrypt {
		for i, file := range flag.Args() {
			filesin, _ := WalkDirectory(file)
			log.Printf("encrypt %d: %d files %v", i, len(filesin), filesin)
			fc.EncryptFiles(filesin, fc.GetCryptedFileName(i))
		}
	}

	if decrypt {
		if err := fc.DecryptFile(flag.Args()[0], fc.GetPlainFileName(flag.Args()[0])); err != nil {
			log.Fatal(err)
		}
	}
}
