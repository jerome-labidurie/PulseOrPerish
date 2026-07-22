package main

import (
	"flag"
	"fmt"
	"os"
	"pulseorperish/internal/fscrypt"
	"pulseorperish/internal/logx"

	"github.com/rs/zerolog/log"
)

// Version, BuildDate, and CommitHash are set during build via ldflags.
var (
	Version    = "dev"
	BuildDate  = "unknown"
	CommitHash = "unknown"
)

// usage prints the help message with detailed explanations of encryption/decryption behavior.
func usage() {
	fmt.Fprintf(os.Stderr, `PoPCrypt - File (En|De)cryption Tool

USAGE:
  popcrypt [flags] -p xxx -e|--encrypt <path>...    Encrypt file(s) or directory(ies)
  popcrypt [flags] -p xxx -d|--decrypt <file.%s>   Decrypt an encrypted archive

ENCRYPTION:
  Files are archived with tar, compressed (gzip or lzw), then encrypted
  with XChaCha20-Poly1305. Output filename: file_NNNN.tar.{gz|lzw}.%s

  Example:
    popcrypt -e -p "mypassword" /path/to/dir1 /path/to/dir2
    → Produces: file_0000.tar.gz.%s, file_0001.tar.gz.%s

DECRYPTION:
  Decrypts the archive to a tar stream (still compressed).
  Output is NOT automatically decompressed. To extract:
    popcrypt -d -p "mypassword" file_0000.tar.gz.%s
    → Produces: file_0000.tar.gz
    → Then extract with: tar -xvzf file_0000.tar.gz
	(for lzw, use uncompress then tar -xvf)

FLAGS:
`, fscrypt.FileExtension, fscrypt.FileExtension, fscrypt.FileExtension, fscrypt.FileExtension, fscrypt.FileExtension)
	flag.PrintDefaults()
}

func main() {
	var decrypt bool
	var encrypt bool
	var passwordStr string

	help := flag.Bool("help", false, "Display help")
	flag.StringVar(&passwordStr, "password", "", "Password to encrypt/decrypt")
	flag.StringVar(&passwordStr, "p", "", "Alias for -password")
	flag.BoolVar(&decrypt, "decrypt", false, "Decrypt a file")
	flag.BoolVar(&decrypt, "d", false, "Alias for -decrypt")
	flag.BoolVar(&encrypt, "encrypt", false, "Encrypt files")
	flag.BoolVar(&encrypt, "e", false, "Alias for -encrypt")
	compressor := flag.String("comp", "gz", "Compression to use: gz|lzw")
	version := flag.Bool("version", false, "Display version")
	debug := flag.Bool("debug", false, "Set log level to debug")

	flag.Usage = usage
	flag.Parse()

	level := "info"
	if *debug {
		level = "debug"
	}
	logger, _, err := logx.New(level, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		os.Exit(2)
	}

	if *help {
		usage()
		os.Exit(0)
	}

	if *version {
		fmt.Printf("PoPCrypt version %s (built %s, commit %s)\n", Version, BuildDate, CommitHash)
		os.Exit(0)
	}

	if passwordStr == "" {
		usage()
		log.Fatal().Msg("password is required")
	}
	if encrypt && decrypt {
		usage()
		log.Fatal().Msg("Cannot encrypt and decrypt")
	}
	if encrypt && len(flag.Args()) < 1 {
		usage()
		log.Fatal().Msg("-encrypt needs files")
	}
	if decrypt && len(flag.Args()) != 1 {
		usage()
		log.Fatal().Msg("-decrypt needs one file only")
	}
	if *compressor != "gz" && *compressor != "lzw" {
		usage()
		log.Fatal().Msg("Only gz & lzw supported")
	}
	// log.Printf("%v\n", flag.Args())
	fc := fscrypt.FsCrypt{
		Password: []byte(passwordStr),
		Compress: *compressor,
		Logger:   logger,
	}

	var hasError bool
	defer func() {
		fc.Clear()
		if hasError {
			os.Exit(1)
		}
	}()
	fc.Init()

	if encrypt {
		for i, file := range flag.Args() {
			log.Info().Int("index", i).Str("input", file).Msg("Encrypting input")
			if err := fc.EncryptPaths([]string{file}, fc.GetCryptedFileName(i)); err != nil {
				log.Error().Err(err).Msg("encryption failed")
				hasError = true
				continue
			}
		}
	}

	if decrypt {
		if err := fc.DecryptFile(flag.Args()[0], fc.GetPlainFileName(flag.Args()[0])); err != nil {
			log.Error().Err(err).Msg("decryption failed")
			hasError = true
		}
	}
}
