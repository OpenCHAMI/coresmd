package coresmd

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pin/tftp"
)

func startTFTPServer(directory string) {
	s := tftp.NewServer(readHandler(directory), nil)
	err := s.ListenAndServe(":69") // default TFTP port
	if err != nil {
		log.Fatalf("failed to start TFTP server: %v", err)
	}
}

func readHandler(directory string) func(string, io.ReaderFrom) error {
	return func(filename string, rf io.ReaderFrom) error {
		filePath := filepath.Join(directory, filename)
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = rf.ReadFrom(file)
		return err
	}
}
