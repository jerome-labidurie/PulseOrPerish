package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"pulseorperish/internal/fscrypt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	log.Debug().Str("found", strings.Join(files, ","))

	return files, nil
}

func main() {
	var decrypt bool
	var encrypt bool
	var password string

	help := flag.Bool("help", false, "Display help")
	flag.StringVar(&password, "password", "", "Password to encrypt/decrypt")
	flag.StringVar(&password, "p", "", "Alias for -password")
	flag.BoolVar(&decrypt, "decrypt", false, "Decrypt a file")
	flag.BoolVar(&decrypt, "d", false, "Alias for -decrypt")
	flag.BoolVar(&encrypt, "encrypt", false, "Encrypt files")
	flag.BoolVar(&encrypt, "e", false, "Alias for -encrypt")
	compressor := flag.String("comp", "gz", "Compression to use gz|lzw")
	version := flag.Bool("version", false, "Display version")
	debug := flag.Bool("debug", false, "Set log level to debug")

	flag.Parse()

	// set logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *version {
		fmt.Printf("PoPCrypt version %s (built %s, commit %s)\n", Version, BuildDate, CommitHash)
		os.Exit(0)
	}

	if password == "" {
		flag.Usage()
		log.Fatal().Msg("password is required")
	}
	if encrypt && decrypt {
		flag.Usage()
		log.Fatal().Msg("Cannot encrypt and decrypt")
	}
	if encrypt && len(flag.Args()) < 1 {
		flag.Usage()
		log.Fatal().Msg("-encrypt needs files")
	}
	if decrypt && len(flag.Args()) != 1 {
		flag.Usage()
		log.Fatal().Msg("-decrypt needs one file only")
	}
	if *compressor != "gz" && *compressor != "lzw" {
		flag.Usage()
		log.Fatal().Msg("Only gz & lzw supported")
	}
	// log.Printf("%v\n", flag.Args())

	fc := fscrypt.FsCrypt{
		Password: password,
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
			log.Fatal().Msg(err.Error())
		}
	}
}
