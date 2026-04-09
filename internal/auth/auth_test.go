// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Mode
		wantErr bool
	}{
		{name: "empty maps to disabled", in: "", want: ModeDisabled},
		{name: "explicit disabled", in: "disabled", want: ModeDisabled},
		{name: "optional", in: "optional", want: ModeOptional},
		{name: "required", in: "required", want: ModeRequired},
		{name: "trim and case normalize", in: "  OpTional ", want: ModeOptional},
		{name: "invalid", in: "shadow", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMode(%q): expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMode(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewSetsFields(t *testing.T) {
	p := New(Config{Mode: ModeOptional, TokensmithURL: "https://tokensmith.example"}, "bootstrap", nil)
	if p == nil {
		t.Fatal("New() returned nil provider")
	}
	if p.client == nil {
		t.Fatal("New() returned provider with nil client")
	}
	if p.mode != ModeOptional {
		t.Fatalf("provider mode = %q, want %q", p.mode, ModeOptional)
	}
	if p.log == nil {
		t.Fatal("New() should set a default logger when nil is provided")
	}
}

func TestStopSafeAndIdempotent(t *testing.T) {
	var p *Provider
	p.Stop() // nil receiver should be safe

	p = &Provider{}
	p.Stop() // cancel is nil
	p.Stop() // idempotent
}

func TestStartAutoRefreshDisabledNoop(t *testing.T) {
	p := &Provider{mode: ModeDisabled}
	p.StartAutoRefresh()
	if p.cancel != nil {
		t.Fatal("StartAutoRefresh() in disabled mode should not set cancel")
	}
}

func TestGetBearerTokenEmptyStates(t *testing.T) {
	var nilProvider *Provider
	if got := nilProvider.GetBearerToken(); got != "" {
		t.Fatalf("nil provider token = %q, want empty", got)
	}

	p := &Provider{mode: ModeDisabled}
	if got := p.GetBearerToken(); got != "" {
		t.Fatalf("disabled provider token = %q, want empty", got)
	}

	p = &Provider{mode: ModeRequired, client: nil}
	if got := p.GetBearerToken(); got != "" {
		t.Fatalf("nil client token = %q, want empty", got)
	}
}
