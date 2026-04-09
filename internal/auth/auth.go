// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package auth provides optional TokenSmith-backed service authentication for
// outbound SMD requests made by the CoreDNS and CoreDHCP coresmd plugins.
//
// Three modes control behaviour:
//
//   - disabled (default) – no authentication is attempted; existing behaviour
//     is fully preserved.
//   - optional – a bootstrap token exchange is attempted at startup; if it
//     fails the plugin continues unauthenticated and logs the failure.
//   - required – startup fails if the bootstrap token exchange fails.
//
// The bootstrap token is never stored in plugin config files; it must be
// supplied via the TOKENSMITH_BOOTSTRAP_TOKEN environment variable.
package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openchami/tokensmith/pkg/tokenservice"
	"github.com/sirupsen/logrus"
)

// BootstrapTokenEnvVar is the environment variable read at startup for the
// one-time bootstrap token.
const BootstrapTokenEnvVar = tokenservice.BootstrapTokenEnvVar

// Mode controls how the SMD client authenticates outbound requests.
type Mode string

const (
	// ModeDisabled disables authentication. No TokenSmith dependency at runtime.
	ModeDisabled Mode = "disabled"
	// ModeOptional attempts authentication; startup continues unauthenticated on failure.
	ModeOptional Mode = "optional"
	// ModeRequired fails startup if the bootstrap token exchange fails.
	ModeRequired Mode = "required"
)

// ParseMode converts a string to a Mode. An empty string maps to ModeDisabled.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(ModeDisabled), "":
		return ModeDisabled, nil
	case string(ModeOptional):
		return ModeOptional, nil
	case string(ModeRequired):
		return ModeRequired, nil
	default:
		return "", fmt.Errorf("unknown auth_mode %q: must be disabled, optional, or required", s)
	}
}

// Config holds plugin-supplied auth configuration. No secrets belong here;
// the bootstrap token is read from the environment by the caller.
// target_service and scopes are intentionally absent: they are encoded in the
// bootstrap token at mint time and TokenSmith uses those values when the
// fields are omitted from the exchange request.
type Config struct {
	Mode          Mode
	TokensmithURL string
	RefreshBefore time.Duration // defaults to 2 minutes
}

// Provider wraps a tokenservice.ServiceClient and manages its lifecycle for
// the duration of the plugin process.
type Provider struct {
	client *tokenservice.ServiceClient
	mode   Mode
	log    logrus.FieldLogger
	cancel context.CancelFunc
}

// New creates a Provider using cfg and the supplied bootstrapToken.
// bootstrapToken should be the value of $TOKENSMITH_BOOTSTRAP_TOKEN.
// target_service and scopes are read from the bootstrap token by TokenSmith;
// the plugin does not need to supply them.
func New(cfg Config, bootstrapToken string, log logrus.FieldLogger) *Provider {
	if cfg.RefreshBefore <= 0 {
		cfg.RefreshBefore = 2 * time.Minute
	}
	if log == nil {
		log = logrus.NewEntry(logrus.New())
	}

	sc := tokenservice.NewServiceClientWithOptions(
		cfg.TokensmithURL,
		"coresmd",
		"coresmd",
		"coresmd",
		"",
		tokenservice.WithBootstrapToken(bootstrapToken),
		tokenservice.WithRefreshBefore(cfg.RefreshBefore),
	)

	return &Provider{
		client: sc,
		mode:   cfg.Mode,
		log:    log,
	}
}

// Initialize performs the bootstrap token exchange according to the configured Mode.
//
// For optional mode a 15-second timeout is applied; a failed exchange is
// logged but Initialize returns nil so the plugin can start unauthenticated.
//
// For required mode the ServiceClient's built-in retry policy applies (5
// attempts, 1–15 s exponential backoff); a failure returns a non-nil error.
func (p *Provider) Initialize() error {
	if p.mode == ModeDisabled {
		return nil
	}

	if p.mode == ModeOptional {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.client.Initialize(ctx); err != nil {
			p.log.Warnf("coresmd auth: optional token exchange failed, continuing unauthenticated: %v", err)
			return nil
		}
		p.log.Info("coresmd auth: service token obtained (optional mode)")
		return nil
	}

	// required: fail closed
	if err := p.client.Initialize(context.Background()); err != nil {
		return fmt.Errorf("coresmd auth: required token exchange failed: %w", err)
	}
	p.log.Info("coresmd auth: service token obtained (required mode)")
	return nil
}

// StartAutoRefresh spawns a background goroutine that proactively renews the
// service token until the refresh token expires or Stop is called. It is a
// no-op when mode is disabled.
func (p *Provider) StartAutoRefresh() {
	if p.mode == ModeDisabled {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func() {
		p.client.StartAutoRefresh(ctx)
		if p.mode == ModeRequired {
			p.log.Error("coresmd auth: auto-refresh stopped (refresh token expired); SMD requests will be unauthenticated")
		} else {
			p.log.Warn("coresmd auth: auto-refresh stopped (refresh token expired); SMD requests will be unauthenticated")
		}
	}()
}

// Stop cancels the auto-refresh goroutine. Safe to call on a nil Provider.
func (p *Provider) Stop() {
	if p == nil {
		return
	}
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// GetBearerToken returns the current access token string, or "" when no token
// is available (mode disabled, exchange not yet completed, or exchange failed).
// This method satisfies the func() string signature expected by
// smdclient.SmdClient.TokenProvider.
func (p *Provider) GetBearerToken() string {
	if p == nil || p.mode == ModeDisabled || p.client == nil {
		return ""
	}
	st := p.client.GetServiceToken()
	if st == nil {
		return ""
	}
	return st.Token
}
