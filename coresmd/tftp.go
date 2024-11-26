package coresmd

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pin/tftp"
)

const defaultScriptName = "default"

var defaultScript = `#!ipxe
reboot
`

type ScriptReader struct{}

func (sr ScriptReader) Read(b []byte) (int, error) {
	nBytes := copy(b, []byte(defaultScript))
	return nBytes, io.EOF
}

func startTFTPServer(directory string) {
	s := tftp.NewServer(readHandler(directory), nil)
	err := s.ListenAndServe(":69") // default TFTP port
	if err != nil {
		log.Fatalf("failed to start TFTP server: %v", err)
	}
}

func readHandler(directory string) func(string, io.ReaderFrom) error {
	return func(filename string, rf io.ReaderFrom) error {
		if filename == defaultScriptName {
			var sr ScriptReader
			_, err := rf.ReadFrom(sr)
			return err
		}
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
