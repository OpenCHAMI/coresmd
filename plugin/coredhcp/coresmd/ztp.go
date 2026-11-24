// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package coresmd

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

type ztpBoot struct {
	userClass string
	uri       *url.URL
}

// addZTPMapping parses CLASS,URL. Only the first comma separates the fields;
// commas and equals signs inside the URL are preserved.
func (c *Config) addZTPMapping(value string) error {
	class, rawURL, ok := strings.Cut(value, ",")
	if !ok || len(class) == 0 || len(class) > 255 {
		return fmt.Errorf("expected CLASS,URL with a class of 1-255 bytes")
	}
	for _, ch := range []byte(class) {
		if ch < '!' || ch > '~' {
			return fmt.Errorf("class must contain printable ASCII without whitespace")
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if !u.IsAbs() || u.Hostname() == "" {
		return fmt.Errorf("URL must be absolute and include a host")
	}
	for _, boot := range c.ztpBoots {
		if boot.userClass == class {
			return fmt.Errorf("duplicate class %q", class)
		}
	}
	c.ztpBoots = append(c.ztpBoots, ztpBoot{userClass: class, uri: u})
	return nil
}

// ztpURI uses configuration order, independent of the client's class order.
func (c Config) ztpURI(classes []string) *url.URL {
	for _, boot := range c.ztpBoots {
		if slices.Contains(classes, boot.userClass) {
			return boot.uri
		}
	}
	return nil
}
