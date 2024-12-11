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
		var raddr string
		ot, ok := rf.(tftp.OutgoingTransfer)
		if !ok {
			log.Error("unable to get remote address, setting to (unknown)")
			raddr = "(unknown)"
		} else {
			ra := ot.RemoteAddr()
			raptr := &ra
			raddr = raptr.IP.String()
		}
		if filename == defaultScriptName {
			log.Infof("tftp: %s requested default script")
			var sr ScriptReader
			nbytes, err := rf.ReadFrom(sr)
			log.Infof("tftp: sent %d bytes of default script to %s", nbytes, raddr)
			return err
		}
		log.Infof("tftp: %s requested file %s", raddr, filename)
		filePath := filepath.Join(directory, filename)
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		nbytes, err := rf.ReadFrom(file)
		log.Infof("tftp: sent %d bytes of file %s to %s", nbytes, filename, raddr)
		return err
	}
}
