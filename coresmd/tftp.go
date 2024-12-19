package coresmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pin/tftp/v3"
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

// tftpServer configures and starts a TFTP server.
type tftpServer struct {
	address    string
	directory  string
	port       int
	singlePort bool
}

// Start creates, configures, and starts the TFTP server implementation.
func (t *tftpServer) Start() {
	s := tftp.NewServer(readHandler(t.directory), nil)
	if t.singlePort {
		s.EnableSinglePort()
	}
	err := s.ListenAndServe(fmt.Sprintf("%s:%d", t.address, t.port))
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
			log.Infof("tftp: %s requested default script", raddr)
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
