/*
Copyright 2024 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import (
	"fmt"
	"sort"
	"strings"

	"github.com/digitalocean/godo"
)

const (
	missingFirewallLabel = "<n/a>"
	ruleOpeningToken     = "<"
	ruleClosingToken     = ">"
)

func printRelevantFirewallParts(fw *godo.Firewall) string {
	if fw == nil {
		return missingFirewallLabel
	}
	return printFirewallParts(fw.Name, fw.InboundRules, fw.OutboundRules, fw.Tags)
}

func printRelevantFirewallRequestParts(fr *godo.FirewallRequest) string {
	if fr == nil {
		return missingFirewallLabel
	}
	return printFirewallParts(fr.Name, fr.InboundRules, fr.OutboundRules, fr.Tags)
}

func printFirewallParts(name string, inboundRules []godo.InboundRule, outboundRules []godo.OutboundRule, tags []string) string {
	parts := []string{fmt.Sprintf("Name:%s", name)}
	parts = append(parts, fmt.Sprintf("inRules:[%s]", printInboundRules(inboundRules)))
	parts = append(parts, fmt.Sprintf("outRules:[%s]", printOutboundRules(outboundRules)))
	parts = append(parts, fmt.Sprintf("Tags:%s", tags))

	return strings.Join(parts, " ")
}

func printInboundRules(inboundRules []godo.InboundRule) string {
	inbRules := make([]string, 0, len(inboundRules))
	for _, inbRule := range inboundRules {
		inbRules = append(inbRules, printInboundRule(inbRule))
	}
	sort.Strings(inbRules)
	return strings.Join(inbRules, " ")
}

func printInboundRule(inboundRule godo.InboundRule) string {
	portRange := inboundRule.PortRange
	if inboundRule.PortRange == "" || inboundRule.PortRange == "all" {
		portRange = "0"
	}
	rule := fmt.Sprintf("%sProto:%s PortRange:%s", ruleOpeningToken, inboundRule.Protocol, portRange)

	if inboundRule.Sources != nil {
		rule += fmt.Sprintf(" AddrSources:%s", inboundRule.Sources.Addresses)
	}

	rule += ruleClosingToken
	return rule
}

func printOutboundRules(outboundRules []godo.OutboundRule) string {
	outbRules := make([]string, 0, len(outboundRules))
	for _, outbRule := range outboundRules {
		outbRules = append(outbRules, printOutboundRule(outbRule))
	}
	sort.Strings(outbRules)
	return strings.Join(outbRules, " ")
}

func printOutboundRule(outboundRule godo.OutboundRule) string {
	portRange := outboundRule.PortRange
	if outboundRule.PortRange == "" || outboundRule.PortRange == "all" {
		portRange = "0"
	}
	rule := fmt.Sprintf("%sProto:%s PortRange:%s", ruleOpeningToken, outboundRule.Protocol, portRange)

	if outboundRule.Destinations != nil {
		rule += fmt.Sprintf(" AddrDestinations:%s", outboundRule.Destinations.Addresses)
	}

	rule += ruleClosingToken
	return rule
}
