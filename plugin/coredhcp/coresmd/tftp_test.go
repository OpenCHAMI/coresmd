package coresmd

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

//==============================================================================
// TEST HELPERS
//==============================================================================

// recorderReaderFrom implements io.ReaderFrom and records what was read.
type recorderReaderFrom struct {
	buf bytes.Buffer
	n   int64
}

func (r *recorderReaderFrom) ReadFrom(src io.Reader) (int64, error) {
	n, err := io.Copy(&r.buf, src)
	r.n = n
	// io.Copy treats EOF as success, emulate that here.
	if err != nil && err != io.EOF {
		return n, err
	}
	return n, nil
}

// recorderOutgoingTransfer implements both io.ReaderFrom and tftp.OutgoingTransfer.
type recorderOutgoingTransfer struct {
	recorderReaderFrom
	addr net.UDPAddr
}

func (r *recorderOutgoingTransfer) SetSize(n int64) {}

func (r *recorderOutgoingTransfer) RemoteAddr() net.UDPAddr {
	return r.addr
}

//==============================================================================
// ScriptReader Tests
//==============================================================================

func TestScriptReaderRead(t *testing.T) {
	sr := ScriptReader{}

	tests := []struct {
		name     string
		bufSize  int
		wantN    int
		wantData string
	}{
		{
			name:     "buffer smaller than script",
			bufSize:  len(defaultScript) / 2,
			wantN:    len(defaultScript) / 2,
			wantData: defaultScript[:len(defaultScript)/2],
		},
		{
			name:     "buffer exact size",
			bufSize:  len(defaultScript),
			wantN:    len(defaultScript),
			wantData: defaultScript,
		},
		{
			name:     "buffer larger than script",
			bufSize:  len(defaultScript) * 2,
			wantN:    len(defaultScript),
			wantData: defaultScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.bufSize)

			n, err := sr.Read(buf)
			if err != io.EOF {
				t.Fatalf("ScriptReader.Read() error = %v, want io.EOF", err)
			}

			if n != tt.wantN {
				t.Fatalf("ScriptReader.Read() n = %d, want %d", n, tt.wantN)
			}

			if got := string(buf[:n]); got != tt.wantData {
				t.Fatalf("ScriptReader.Read() data = %q, want %q", got, tt.wantData)
			}
		})
	}
}

//==============================================================================
// readHandler Tests
//==============================================================================

func TestReadHandler_DefaultScript(t *testing.T) {
	tests := []struct {
		name       string
		readerFrom io.ReaderFrom
	}{
		{
			name:       "without OutgoingTransfer",
			readerFrom: &recorderReaderFrom{},
		},
		{
			name: "with OutgoingTransfer",
			readerFrom: &recorderOutgoingTransfer{
				addr: net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 69},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := readHandler("/ignored/for/default")

			err := h(defaultScriptName, tt.readerFrom)
			if err != nil {
				t.Fatalf("readHandler(...) returned error = %v, want nil", err)
			}

			// Inspect what was written via ReadFrom.
			var buf bytes.Buffer
			switch rf := tt.readerFrom.(type) {
			case *recorderReaderFrom:
				buf = rf.buf
				if rf.n != int64(len(defaultScript)) {
					t.Fatalf("ReadFrom bytes = %d, want %d", rf.n, len(defaultScript))
				}
			case *recorderOutgoingTransfer:
				buf = rf.buf
				if rf.n != int64(len(defaultScript)) {
					t.Fatalf("ReadFrom bytes = %d, want %d", rf.n, len(defaultScript))
				}
			default:
				t.Fatalf("unexpected readerFrom type %T", tt.readerFrom)
			}

			if got := buf.String(); got != defaultScript {
				t.Fatalf("default script content = %q, want %q", got, defaultScript)
			}
		})
	}
}

func TestReadHandler_FileCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "coresmd-tftp-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a real file to be served.
	const fileName = "testfile.ipxe"
	const fileContent = "#!ipxe\necho hello\n"

	if err := os.WriteFile(filepath.Join(tmpDir, fileName), []byte(fileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name        string
		filename    string
		readerFrom  io.ReaderFrom
		wantErr     bool
		wantContent string
	}{
		{
			name:        "existing file with OutgoingTransfer",
			filename:    fileName,
			readerFrom:  &recorderOutgoingTransfer{},
			wantErr:     false,
			wantContent: fileContent,
		},
		{
			name:       "missing file",
			filename:   "no_such_file.ipxe",
			readerFrom: &recorderOutgoingTransfer{},
			wantErr:    true,
		},
		{
			name:        "existing file without OutgoingTransfer",
			filename:    fileName,
			readerFrom:  &recorderReaderFrom{},
			wantErr:     false,
			wantContent: fileContent,
		},
	}

	h := readHandler(tmpDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h(tt.filename, tt.readerFrom)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("readHandler(...) error = nil, want non-nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("readHandler(...) error = %v, want nil", err)
			}

			// Inspect what was written via ReadFrom.
			switch rf := tt.readerFrom.(type) {
			case *recorderReaderFrom:
				if got := rf.buf.String(); got != tt.wantContent {
					t.Fatalf("served content = %q, want %q", got, tt.wantContent)
				}
			case *recorderOutgoingTransfer:
				if got := rf.buf.String(); got != tt.wantContent {
					t.Fatalf("served content = %q, want %q", got, tt.wantContent)
				}
			default:
				t.Fatalf("unexpected readerFrom type %T", tt.readerFrom)
			}
		})
	}
}
