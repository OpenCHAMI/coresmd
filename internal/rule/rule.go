// SPDX-FileCopyrightText: Copyright 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package rule

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/rfc1035label"
	"github.com/sirupsen/logrus"

	"github.com/openchami/coresmd/internal/iface"
	"github.com/openchami/coresmd/internal/parse"
)

const DefaultPattern = "unknown-{04d}"

var AllowedKeys = []string{
	"cidr",
	"continue",
	"domain",
	"domain_append",
	"hostname",
	"id",
	"id_set",
	"ignore",
	"log",
	"name",
	"netmask",
	"routers",
	"subnet",
	"type",
}

// KeyAllowed returns true if the passed key is an allowed rule key.
func KeyAllowed(key string) bool {
	for _, k := range AllowedKeys {
		if k == key {
			return true
		}
	}
	return false
}

// Rule represents a hostname rule
type Rule struct {
	Name   string // ID of rule
	Log    string // "info" (log match), "debug" (log match/skip), "none"/omit (don't log)
	Match  Match  // criteria for rule to match host
	Action Action // what to do if rule matches host
}

func (r Rule) String() string {
	return fmt.Sprintf("name:%s,log:%s,%s,%s",
		r.Name,
		r.Log,
		r.Match,
		r.Action,
	)
}

// MatchIface evaluates the rule for the passed interface and determines whether
// the rule matches and whether the evaluation should continue down the chain.
func (r Rule) MatchIface(ii iface.IfaceInfo) (matches, cont bool) {
	// Determine whether to continue
	cont = r.Action.Continue

	matchSet := make(map[string]bool)
	if r.Match.Types != nil && len(r.Match.Types) > 0 {
		matchSet["types"] = true
	}
	if len(r.Match.Subnets) > 0 {
		matchSet["subnets"] = true
	}
	if id := strings.TrimSpace(r.Match.ID); id != "" {
		matchSet["id"] = true
		r.Match.ID = id
	} else if r.Match.IDSet != nil {
		matchSet["id_set"] = true
	}

	matchCounter := 0

	// Match type
	if _, ok := matchSet["types"]; ok {
		if _, ok := r.Match.Types[ii.Type]; ok {
			matchCounter++
		}
	}

	// Match subnet
	if _, ok := matchSet["subnets"]; ok && len(ii.IPList) > 0 {
		for _, subnet := range r.Match.Subnets {
			if subnet == nil {
				continue
			}
			// We assume that the first IP will be assigned
			if subnet.Contains(ii.IPList[0]) {
				matchCounter++
				break
			}
		}
	}

	if _, ok := matchSet["id"]; ok {
		// Match ID
		if ii.CompID == r.Match.ID {
			matchCounter++
		}
	} else if _, ok := matchSet["id_set"]; ok {
		// Match ID set
		if r.Match.IDSet.Match(ii.CompID) {
			matchCounter++
		}
	}

	// Tally up the matches, returning true if everything matches
	if matchCounter == len(matchSet) {
		matches = true
	}

	return
}

// Match represents criteria for hostname rule to match host
type Match struct {
	Types   map[string]bool // any type in map matches, all types if empty
	Subnets []*net.IPNet    // any subnet in slice matches, all subnets if empty
	ID      string          // xname to match, any if empty
	IDSet   IDSetMatcher    // set of xnames to match, any if nil
}

func (m Match) String() string {
	var matchStr string

	if m.Types != nil {
		keys := make([]string, 0, len(m.Types))
		for typ := range m.Types {
			keys = append(keys, typ)
		}
		sort.Strings(keys)
		typStr := ",types:" + strings.Join(keys, "|")
		matchStr += typStr
	}

	if m.Subnets != nil {
		var notfirst bool
		subnetStr := ",subnets:"
		for _, subnet := range m.Subnets {
			if notfirst {
				subnetStr += fmt.Sprintf("|%s", subnet)
			} else {
				subnetStr += fmt.Sprintf("%s", subnet)
				notfirst = true
			}
		}
		matchStr += subnetStr
	}

	if id := strings.TrimSpace(m.ID); id != "" {
		matchStr += fmt.Sprintf(",id:%s", id)
	}

	if m.IDSet != nil {
		matchStr += fmt.Sprintf(",id_set:%s", m.IDSet)
	}

	return strings.TrimLeft(matchStr, ",")
}

// Action represents an action to take upon a rule matching
type Action struct {
	Hostname     string     // hostname pattern to apply
	Domain       string     // rule-specific domain to use when generating the FQDN
	DomainAppend string     // controls when/how to append domain to hostname (rule vs. global vs. both)
	Netmask      net.IPMask // rule-specific network mask for IPv4
	Routers      []net.IP   // router IPs for selected component(s)
	Continue     bool       // whether to continue parsing subsequent rules if this matches
	Ignore       bool       // if true, drop DHCP request without responding (takes precedence over all other actions)
}

func (a Action) String() string {
	var actionStr string

	hn := strings.TrimSpace(a.Hostname)
	if hn != "" {
		actionStr += fmt.Sprintf(",hostname:%s", hn)
	}

	dom := strings.TrimSpace(a.Domain)
	if dom != "" {
		actionStr += fmt.Sprintf(",domain:%s", dom)
	}

	if da := strings.TrimSpace(a.DomainAppend); da != "" {
		actionStr += fmt.Sprintf(",domain_append:%s", da)
	}
	actionStr += fmt.Sprintf(",continue:%v", a.Continue)
	actionStr += fmt.Sprintf(",ignore:%v", a.Ignore)

	// Canonicalize netmask for IPv4 to CIDR notation
	if ones, _ := a.Netmask.Size(); ones > 0 {
		actionStr += fmt.Sprintf(",netmask:/%d", ones)
	}

	if len(a.Routers) > 0 {
		parts := make([]string, 0, len(a.Routers))
		for _, r := range a.Routers {
			parts = append(parts, r.String())
		}
		actionStr += fmt.Sprintf(",routers:%s", strings.Join(parts, "|"))
	}

	return strings.TrimLeft(actionStr, ",")
}

// IDSetMatcher provides a function for matching a component ID to a defined set
// of IDs.
type IDSetMatcher interface {
	Match(id string) bool
	String() string // for debugging purposes
}

// CompileIDSet creates an IDSetMatcher from a string expression representing
// the set of IDs.
func CompileIDSet(expr string) (IDSetMatcher, error) {
	// TODO: implement
	return nil, fmt.Errorf("CompileIDSet() is not implemented: %q", expr)
}

// ParseRule parses a string representing a hostname rule (everything to the
// right of 'hostname_rule=') and returns a representative Rule. Within the
// rule string, key/value delimiters are ':' (for example: type:Node).
func ParseRule(rule string) (Rule, error) {
	comps, err := createRuleCompDict(rule)
	if err != nil {
		return Rule{}, err
	}

	var (
		r Rule
		m Match
		a Action
	)

	// log (optional)
	if log, ok := comps["log"]; ok && log != "" {
		switch log {
		case "info", "debug", "none":
			r.Log = log
		default:
			return Rule{}, NewErrInvalidValue("log", log, "'info', 'debug', or 'none'")
		}
	} else {
		// Inherit from global rule_log. The caller is responsible for setting
		// the effective default (e.g. to the global rule_log value) if desired.
		r.Log = ""
	}

	// name, rule identifier (generated below if not set)
	if name, ok := comps["name"]; ok && name != "" {
		r.Name = name
	}

	// hostname (action)
	if hn, ok := comps["hostname"]; ok && hn != "" {
		a.Hostname = hn
	}

	// router (action)
	if rtrs, ok := comps["routers"]; ok && rtrs != "" {
		for _, r := range strings.Split(rtrs, "|") {
			ip := net.ParseIP(strings.TrimSpace(r))
			if ip == nil {
				return Rule{}, NewErrInvalidValue("routers", r, "valid IPv4 address")
			}
			ip4 := ip.To4()
			if ip4 == nil {
				return Rule{}, NewErrInvalidValue("routers", r, "valid IPv4 address")
			}
			a.Routers = append(a.Routers, ip4)
		}
	}

	// netmask/cidr (action)
	netmaskstr, netmaskFound := comps["netmask"]
	cidrstr, cidrFound := comps["cidr"]
	if netmaskFound && cidrFound {
		// Both netmask and cidr are not allowed, only one or the other
		return Rule{}, NewErrMutualExclusion("netmask", "cidr")
	} else if netmaskFound {
		// netmask specified

		// Check that mask is a valid IP
		netmaskIP := net.ParseIP(netmaskstr)
		if netmaskIP == nil {
			return Rule{}, NewErrInvalidValue("netmask", netmaskstr, "valid IPv4 address")
		}

		// Check that mask is not unspecified IP like 0.0.0.0
		if netmaskIP.IsUnspecified() {
			return Rule{}, NewErrInvalidValue("netmask", netmaskstr, "specific, usable IPv4 address")
		}

		// Check that mask IP is a valid IPv4
		netmaskIP = netmaskIP.To4()
		if netmaskIP == nil {
			return Rule{}, NewErrInvalidValue("netmask", netmaskstr, "valid IPv4 address")
		}

		// Check that mask IP is a valid IPv4 netmask
		netmask := net.IPv4Mask(netmaskIP[0], netmaskIP[1], netmaskIP[2], netmaskIP[3])
		if !checkValidNetmask(netmask) {
			return Rule{}, NewErrInvalidValue("netmask", netmaskstr, "valid IPv4 netmask")
		}

		a.Netmask = netmask
	} else if cidrFound {
		// cidr specified

		// Convert from string to integer
		cidr, err := strconv.Atoi(cidrstr)
		if err != nil {
			return Rule{}, NewErrInvalidValue("cidr", cidrstr, "integer")
		}

		// Check range validity (32 bits for IPv4)
		if cidr <= 0 || cidr > 32 {
			return Rule{}, NewErrInvalidValue("cidr", cidrstr, "valid IPv4 bit width between 1 and 32, inclusive")
		}

		a.Netmask = net.CIDRMask(cidr, 32)
	}

	// domain override (optional)
	if dom, ok := comps["domain"]; ok && dom != "" {
		// domain:none doesn't make sense when domain_append:none is intended,
		// use it instead
		if strings.EqualFold(strings.TrimSpace(dom), "none") {
			return Rule{}, NewErrInvalidValue("domain", dom, "a domain name (or use domain_append:none)")
		}
		a.Domain = dom
	}

	// continue (optional)
	if cont, ok := comps["continue"]; ok && cont != "" {
		if b, err := parse.ParseBoolLoose(cont); err != nil {
			return Rule{}, NewErrInvalidValue("continue", cont, "boolean")
		} else {
			a.Continue = b
		}
	}

	// ignore (optional, defaults to false)
	if ignore, ok := comps["ignore"]; ok && ignore != "" {
		if b, err := parse.ParseBoolLoose(ignore); err != nil {
			return Rule{}, NewErrInvalidValue("ignore", ignore, "boolean")
		} else {
			a.Ignore = b
		}
	}

	// domain_append (optional)
	//
	// Valid values:
	//   - "global"      - final hostname: <hostname>.<global_domain>
	//   - "rule"        - final hostname: <hostname>.<rule_domain>
	//   - "global,rule" - final hostname: <hostname>.<global_domain>.<rule_domain>
	//   - "none"        - final hostname: <hostname>
	//
	// When omitted, CoreSMD applies the default behavior:
	//   - if global domain is set and rule domain is unset: append global domain
	//   - if global domain is set and rule domain is set: append rule domain (overriding global)
	//   - if global domain is unset and rule domain is unset: leave hostname alone
	//   - if global domain is unset and rule domain is set: append rule domain
	if domapp, ok := comps["domain_append"]; ok && domapp != "" {
		if norm, err := normalizeDomainAppend(domapp); err != nil {
			return Rule{}, NewErrInvalidValue("domain_append", domapp, "'global', 'rule', 'global|rule', 'rule|global', or 'none'")
		} else {
			a.DomainAppend = norm
		}
	}

	// match by type (optional; multivalue)
	//
	// Examples:
	//  - type:Node                    # single type
	//  - type:Node|NodeBMC|HSNSwitch  # multiple types
	if matchType, ok := comps["type"]; ok && strings.TrimSpace(matchType) != "" {
		m.Types = make(map[string]bool)
		for _, t := range strings.Split(matchType, "|") {
			if tt := strings.TrimSpace(t); tt == "" {
				continue
			} else {
				m.Types[tt] = true
			}
		}
		// 'type' requires at least one type, err if none specified
		if len(m.Types) == 0 {
			return Rule{}, NewErrInvalidValue("type", matchType, "at least one type")
		}
	} else if ok {
		// 'type' present but was empty/whitespace
		return Rule{}, NewErrInvalidValue("type", matchType, "at least one type")
	}

	// match by subnet (optional; multivalue)
	//
	// Examples:
	//  - subnet:172.16.0.0/24                # single subnet
	//  - subnet:172.16.0.0/24|172.16.1.0/21  # multiple subnets
	if matchSubnet, ok := comps["subnet"]; ok && matchSubnet != "" {
		for _, s := range strings.Split(matchSubnet, "|") {
			if _, ipnet, err := net.ParseCIDR(strings.TrimSpace(s)); err != nil {
				return Rule{}, NewErrInvalidValue("subnet", s, "network subnet (e.g. 172.16.0.0/21)")
			} else {
				m.Subnets = append(m.Subnets, ipnet)
			}
		}

		// If subnet/cidr not defined, use the subnet's CIDR for it
		if ones, size := a.Netmask.Size(); ones == 0 || size == 0 {
			cidr, _ := m.Subnets[0].Mask.Size()
			if cidr > 0 {
				a.Netmask = m.Subnets[0].Mask
			} else {
				// Do not err if mask is 0 or invalid; fallback to already-set
				// netmask.
			}
		}
	}

	// At least one action is required.
	//
	// Netmask can be set explicitly via netmask/cidr or implicitly from subnet.
	// In this way, subnet can be considered an "action" and is the only match
	// key to be considered such.
	//
	// The ignore action is also valid as a standalone action.
	if strings.TrimSpace(a.Hostname) == "" &&
		len(a.Routers) == 0 &&
		!a.Ignore {
		if ones, size := a.Netmask.Size(); ones == 0 || size == 0 {
			return Rule{}, NewErrRequiredKeys("hostname", "routers", "netmask", "ignore")
		}
	}

	// match by single ID (optional)
	// mutually exclusive with matching by ID set
	if id, ok := comps["id"]; ok && id != "" {
		m.ID = id
	}

	// match by ID set (optional)
	// mutually exclusive with matching by ID
	if idset, ok := comps["id_set"]; ok && idset != "" {
		if matcher, err := CompileIDSet(idset); err != nil {
			return Rule{}, NewErrInvalidValue("id_set", idset, "valid ID set (e.g. x1000s0c0b[0-3]n0)")
		} else {
			m.IDSet = matcher
		}
	}

	// ensure id and id_set mutual exclusion
	if m.ID != "" && m.IDSet != nil {
		return Rule{}, NewErrMutualExclusion("id", "id_set")
	}

	r.Match = m
	r.Action = a

	// Generate rule name if it wasn't set
	if n := strings.TrimSpace(r.Name); n == "" {
		sum := sha256.Sum256([]byte(r.String()))
		r.Name = fmt.Sprintf("rule-%08x", sum[:4])
	}

	return r, nil
}

// normalizeDomainAppend validates and normalizes the domain_append value.
//
// Accepted inputs (case-insensitive, whitespace-tolerant):
//   - "global"
//   - "rule"
//   - "global|rule"
//   - "rule|global"
//   - "none"
//
// Order matters: "global|rule" and "rule|global" are distinct.
//
// Returned values are normalized to lowercase with no whitespace and preserved
// order: "global", "rule", "global|rule", "rule|global", or "none".
func normalizeDomainAppend(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", fmt.Errorf("empty")
	}

	parts := strings.Split(raw, "|")
	var hasGlobal, hasRule, hasNone bool
	order := make([]string, 0, 2)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch p {
		case "global":
			if hasGlobal {
				return "", fmt.Errorf("duplicate token %q", p)
			}
			hasGlobal = true
			order = append(order, "global")
		case "rule":
			if hasRule {
				return "", fmt.Errorf("duplicate token %q", p)
			}
			hasRule = true
			order = append(order, "rule")
		case "none":
			if hasNone {
				return "", fmt.Errorf("duplicate token %q", p)
			}
			hasNone = true
			order = append(order, "none")
		default:
			return "", fmt.Errorf("invalid token %q", p)
		}
	}

	if hasNone {
		// none cannot be combined with anything else
		if hasGlobal || hasRule {
			return "", fmt.Errorf("'none' cannot be combined with other values")
		}
		return "none", nil
	}

	if !hasGlobal && !hasRule {
		return "", fmt.Errorf("no valid values")
	}
	if hasGlobal && hasRule {
		// Preserve order as supplied.
		// order contains both tokens once each.
		return strings.Join(order, "|"), nil
	}
	if hasGlobal {
		return "global", nil
	}
	return "rule", nil
}

// Evaluate4 takes interface information from a DHCPv4 request and a list of
// rules to evaluate and modifies the passed DHCPv4 response according to the
// rules.
//
// A logger object is passed for logging function operations. If nil, it will be
// initialized with default values.
//
// The global settings globalDomain and ruleLog are also passed for effecting
// rule evaluation behavior.
//
// Returns true if a DHCP response should be sent, false if the request should
// be dropped (due to an ignore action).
func Evaluate4(logger *logrus.Entry, ii iface.IfaceInfo, globalDomain, ruleLog string, resp *dhcpv4.DHCPv4, rules []Rule) bool {
	// Init default logger if unset
	if logger == nil {
		logger = logrus.NewEntry(logrus.New())
	}
	logMismatch := func(idx int, rule Rule) {
		var loggingEnabled bool
		switch ruleLog {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("rule_log", ruleLog, "'debug', 'info', or 'none'")
			logger.Error(err)
		}
		switch rule.Log {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("log", rule.Log, "'debug', 'info', or 'none'")
			logger.Errorf("rule[%d] (%s) %v", idx, rule.Name, err)
		}
		if loggingEnabled {
			logger.WithFields(logrus.Fields{
				"comp_id":   ii.CompID,
				"comp_nid":  ii.CompNID,
				"comp_type": ii.Type,
				"mac":       ii.MAC,
				"ips":       ii.IPList,
			}).Infof("rule[%d] (%s) did not match", idx, rule.Name)
		}
	}
	logMatch := func(idx int, rule Rule) {
		var loggingEnabled bool
		switch ruleLog {
		case "debug", "info", "":
			loggingEnabled = true
		case "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("rule_log", ruleLog, "'debug', 'info', or 'none'")
			logger.Error(err)
		}
		switch rule.Log {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("log", rule.Log, "'debug', 'info', or 'none'")
			logger.Errorf("rule[%d] (%s) %v", idx, rule.Name, err)
		}
		if loggingEnabled {
			logger.WithFields(logrus.Fields{
				"comp_id":   ii.CompID,
				"comp_nid":  ii.CompNID,
				"comp_type": ii.Type,
				"mac":       ii.MAC,
				"ips":       ii.IPList,
			}).Infof("rule[%d] (%s) matched", idx, rule.Name)
		}
	}

	for idx, rule := range rules {
		matches, cont := rule.MatchIface(ii)
		if matches {
			logMatch(idx, rule)

			//
			// PERFORM ACTIONS HERE
			//

			// Check if request should be ignored (takes precedence over all other actions)
			if rule.Action.Ignore {
				// Log according to rule's log level
				var loggingEnabled bool
				switch rule.Log {
				case "info", "debug":
					loggingEnabled = true
				case "", "none":
					// Inherit from global or suppress
					if ruleLog == "info" || ruleLog == "debug" {
						loggingEnabled = true
					}
				}
				if loggingEnabled {
					logger.WithFields(logrus.Fields{
						"comp_id":   ii.CompID,
						"comp_type": ii.Type,
						"mac":       ii.MAC,
					}).Infof("rule[%d] (%s) matched with ignore=true, dropping DHCP request", idx, rule.Name)
				}
				return false // Do not send DHCP response to client
			}

			// Set hostname
			if hn := strings.TrimSpace(rule.Action.Hostname); hn != "" {
				resp.Options.Update(dhcpv4.OptHostName(lookupHostname(hn, globalDomain, ii, rule)))
			}

			// Set routers
			if len(rule.Action.Routers) > 0 {
				resp.Options.Update(dhcpv4.OptRouter(rule.Action.Routers...))
			}

			// Set netmask (DHCP option 1)
			//
			// If the rule did not explicitly set a netmask/cidr action but the rule
			// matched on subnet, implicitly set the subnet mask to the CIDR of the
			// first subnet in the match list.
			mask := rule.Action.Netmask
			if ones, size := mask.Size(); ones == 0 || size == 0 {
				if len(rule.Match.Subnets) > 0 {
					m := rule.Match.Subnets[0].Mask
					// Only apply IPv4 masks here (DHCPv4 option 1).
					if len(m) == net.IPv4len {
						mask = m
					}
				}
			}
			if ones, size := mask.Size(); ones != 0 && size != 0 {
				resp.Options.Update(dhcpv4.OptSubnetMask(mask))
			}

			if !cont {
				// Continue not specified for match, so stop here
				break
			}
		} else {
			logMismatch(idx, rule)
		}
	}

	//
	// DEFAULT ACTIONS GO HERE
	//

	// If no hostname was set, fall back to default pattern and global domain
	if len(bytes.TrimSpace(resp.Options.Get(dhcpv4.OptionHostName))) == 0 {
		resp.Options.Update(dhcpv4.OptHostName(lookupHostname(DefaultPattern, globalDomain, ii, Rule{})))
	}

	return true // Send DHCP response to client
}

// Evaluate6 takes interface information from a DHCPv6 request and a list of
// rules to evaluate and modifies the passed DHCPv6 response according to the
// rules.
//
// A logger object is passed for logging function operations. If nil, it will be
// initialized with default values.
//
// The global settings globalDomain and ruleLog are also passed for effecting
// rule evaluation behavior.
//
// Returns true if a DHCP response should be sent, false if the request should
// be dropped (due to an ignore action).
func Evaluate6(logger *logrus.Entry, ii iface.IfaceInfo, globalDomain, ruleLog string, resp *dhcpv6.Message, rules []Rule) bool {
	// Init default logger if unset
	if logger == nil {
		logger = logrus.NewEntry(logrus.New())
	}
	logMismatch := func(idx int, rule Rule) {
		var loggingEnabled bool
		switch ruleLog {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("rule_log", ruleLog, "'debug', 'info', or 'none'")
			logger.Error(err)
		}
		switch rule.Log {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("log", rule.Log, "'debug', 'info', or 'none'")
			logger.Errorf("rule[%d] (%s) %v", idx, rule.Name, err)
		}
		if loggingEnabled {
			logger.WithFields(logrus.Fields{
				"comp_id":   ii.CompID,
				"comp_nid":  ii.CompNID,
				"comp_type": ii.Type,
				"mac":       ii.MAC,
				"ips":       ii.IPList,
			}).Infof("rule[%d] (%s) did not match", idx, rule.Name)
		}
	}
	logMatch := func(idx int, rule Rule) {
		var loggingEnabled bool
		switch ruleLog {
		case "debug", "info", "":
			loggingEnabled = true
		case "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("rule_log", ruleLog, "'debug', 'info', or 'none'")
			logger.Error(err)
		}
		switch rule.Log {
		case "debug":
			loggingEnabled = true
		case "", "info", "none":
			// Do nothing
		default:
			err := NewErrInvalidValue("log", rule.Log, "'debug', 'info', or 'none'")
			logger.Errorf("rule[%d] (%s) %v", idx, rule.Name, err)
		}
		if loggingEnabled {
			logger.WithFields(logrus.Fields{
				"comp_id":   ii.CompID,
				"comp_nid":  ii.CompNID,
				"comp_type": ii.Type,
				"mac":       ii.MAC,
				"ips":       ii.IPList,
			}).Infof("rule[%d] (%s) matched", idx, rule.Name)
		}
	}

	for idx, rule := range rules {
		matches, cont := rule.MatchIface(ii)
		if matches {
			logMatch(idx, rule)

			//
			// PERFORM ACTIONS HERE
			//

			// Check if request should be ignored (takes precedence over all other actions)
			if rule.Action.Ignore {
				// Log according to rule's log level
				var loggingEnabled bool
				switch rule.Log {
				case "info", "debug":
					loggingEnabled = true
				case "", "none":
					// Inherit from global or suppress
					if ruleLog == "info" || ruleLog == "debug" {
						loggingEnabled = true
					}
				}
				if loggingEnabled {
					logger.WithFields(logrus.Fields{
						"comp_id":   ii.CompID,
						"comp_type": ii.Type,
						"mac":       ii.MAC,
					}).Infof("rule[%d] (%s) matched with ignore=true, dropping DHCPv6 request", idx, rule.Name)
				}
				return false // Do not send a DHCP response to client
			}

			// Set hostname
			if hn := strings.TrimSpace(rule.Action.Hostname); hn != "" {
				hname := lookupHostname(hn, globalDomain, ii, rule)
				labels := &rfc1035label.Labels{Labels: strings.Split(hname, ".")}
				resp.UpdateOption(&dhcpv6.OptFQDN{Flags: 0, DomainName: labels})
			}

			if !cont {
				// Continue not specified for match, so stop here
				break
			}
		} else {
			logMismatch(idx, rule)
		}
	}

	//
	// DEFAULT ACTIONS GO HERE
	//

	// If no hostname was set, fall back to default pattern and global domain
	opt := resp.GetOneOption(dhcpv6.OptionFQDN)
	if opt == nil || strings.TrimSpace(opt.String()) == "" {
		hname := lookupHostname(DefaultPattern, globalDomain, ii, Rule{})
		labels := &rfc1035label.Labels{Labels: strings.Split(hname, ".")}
		resp.UpdateOption(&dhcpv6.OptFQDN{Flags: 0, DomainName: labels})
	}

	return true // Send DHCP response to client
}

// createRuleCompDict parses a rule string and creates a map of each rule
// component's key mapped to its value. Duplicate keys are not allowed.
// Rule elements are formatted as key:val (not key=val).
func createRuleCompDict(rule string) (map[string]string, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, ErrEmptyRule
	}

	// Split comma-separated values into rule components
	parts, err := parse.SplitCSV(rule)
	if err != nil {
		return nil, err
	}

	// Parse each rule component (key:val) and place into dictionary
	comps := make(map[string]string, len(parts))
	for idx, p := range parts {
		// Skip empty space
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Split key:val
		key, val, ok := strings.Cut(p, ":")
		if !ok {
			return nil, NewErrKeyValFormat(idx, p)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, NewErrNoKey(idx, p)
		} else if !KeyAllowed(key) {
			return nil, NewErrUnknownKey(idx, key)
		}
		val = strings.TrimSpace(val)
		val, err := parse.Unquote(val)
		if err != nil {
			return nil, NewErrBadQuote(idx, p, err)
		}

		// Disallow duplicate keys
		if _, exists := comps[key]; exists {
			return nil, NewErrDuplicateKey(idx, p, key)
		}

		comps[key] = val
	}

	return comps, nil
}
