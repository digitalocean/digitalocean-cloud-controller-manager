/*
Copyright 2017 DigitalOcean

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
	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/context"
	"strconv"
	"strings"
)

type firewallRequest struct {
	firewallID string
	rules      *godo.FirewallRulesRequest
}

func checkIfPortInRange(port int, fwPortRange string) bool {

	fwPortList := strings.Split(fwPortRange, "-")
	if fwPortList[0] == "all" {
		return true
	}
	if len(fwPortList) == 1 {
		fwPort, _ := strconv.Atoi(fwPortList[0])
		if port == fwPort {
			return true
		}
	} else {
		fwLowerVal, _ := strconv.Atoi(fwPortList[0])
		fwUpperVal, _ := strconv.Atoi(fwPortList[1])
		if port >= fwLowerVal && port <= fwUpperVal {
			return true
		}
	}

	return false

}

func checkIfPortRequiresDeletion(port int, fwPortRange string) bool {

	fwPortList := strings.Split(fwPortRange, "-")
	if fwPortList[0] == "all" {
		return false
	}
	if len(fwPortList) == 1 {
		fwPort, _ := strconv.Atoi(fwPortList[0])
		if port == fwPort {
			return true
		}
	}

	return false

}

func createRuleRequest(sourcelb string, targetPort int) *godo.FirewallRulesRequest {
	rr := &godo.FirewallRulesRequest{
		InboundRules: []godo.InboundRule{
			{
				Protocol:  "tcp",
				PortRange: strconv.Itoa(targetPort),
				Sources: &godo.Sources{
					LoadBalancerUIDs: []string{sourcelb},
				},
			},
		},
	}
	return rr
}

// Check if a firewall rule already exists for a load balancer
// forwarding rule ( returns true if covered by ALL, a port range or a single port)
func lbfwRuleExists(lbID string, lbRule godo.ForwardingRule, firewalls []godo.Firewall) bool {
	ruleExists := false
	//check each firewall to see if rule already exists
	for _, firewall := range firewalls {
		for _, fwInboundRule := range firewall.InboundRules {
			for _, fwSourcelbs := range fwInboundRule.Sources.LoadBalancerUIDs {
				if fwSourcelbs == lbID {
					res := checkIfPortInRange(lbRule.TargetPort, fwInboundRule.PortRange)
					if res && fwInboundRule.Protocol == "tcp" {
						ruleExists = true
					}
				}
			}
		}
	}

	return ruleExists

}

func requestExists(requests []firewallRequest, firewallID string, lbID string, targetPort int) bool {

	for _, request := range requests {
		if firewallID == request.firewallID &&
			lbID == request.rules.InboundRules[0].Sources.LoadBalancerUIDs[0] &&
			strconv.Itoa(targetPort) == request.rules.InboundRules[0].PortRange {
			return true
		}
	}

	return false
}

// Check if a single port rule exists for a load balancer
// forwarding rule ( will not remove ranges defined by ports or ALL )
func lbfwRuleRequiresDeletion(lbID string, lbRule godo.ForwardingRule, firewalls []godo.Firewall) bool {
	requiresDeletion := false
	//check each firewall to see if rule need deleting
	for _, firewall := range firewalls {
		for _, fwInboundRule := range firewall.InboundRules {
			for _, fwSourcelbs := range fwInboundRule.Sources.LoadBalancerUIDs {
				if fwSourcelbs == lbID {
					res := checkIfPortRequiresDeletion(lbRule.TargetPort, fwInboundRule.PortRange)
					if res && fwInboundRule.Protocol == "tcp" {
						requiresDeletion = true
					}
				}
			}
		}
	}

	return requiresDeletion

}

func addRules(client *godo.Client, rulesToAdd []firewallRequest) error {
	for _, ruleToAdd := range rulesToAdd {
		_, err := client.Firewalls.AddRules(context.TODO(), ruleToAdd.firewallID, ruleToAdd.rules)
		if err != nil {
			return fmt.Errorf("Error adding rule to firewall %s", ruleToAdd.firewallID)
		}
	}

	return nil
}

func deleteRules(client *godo.Client, rulesToDelete []firewallRequest) error {
	for _, ruleToDelete := range rulesToDelete {
		_, err := client.Firewalls.RemoveRules(context.TODO(), ruleToDelete.firewallID, ruleToDelete.rules)
		if err != nil {
			return fmt.Errorf("Error removing rule from firewall %s", ruleToDelete.firewallID)
		}
	}

	return nil
}

// EnsureFWRuleExists ensures that the node firewalls have the correct rules for
// the load balancer.
//
// EnsureFWRuleExists will not modify service or nodes.
func (l *loadbalancers) EnsureFWRuleExists(lb *godo.LoadBalancer) ([]firewallRequest, error) {
	var rulesToAdd []firewallRequest

	for _, dropletID := range lb.DropletIDs {
		firewalls, _, err := l.client.Firewalls.ListByDroplet(context.TODO(), dropletID, nil)
		if err != nil {
			return nil, fmt.Errorf("Error listing firewalls for droplet %d", dropletID)
		}

		if len(firewalls) == 0 {
			continue
		}

		for _, lbRule := range lb.ForwardingRules {
			if lbfwRuleExists(lb.ID, lbRule, firewalls) == false &&
				requestExists(rulesToAdd, firewalls[0].ID, lb.ID, lbRule.TargetPort) == false {

				rulesToAdd = append(rulesToAdd, firewallRequest{firewalls[0].ID, createRuleRequest(lb.ID, lbRule.TargetPort)})

			}
		}

	}

	err := addRules(l.client, rulesToAdd)
	if err != nil {
		return rulesToAdd, fmt.Errorf("Error adding firewall rules")
	}

	return rulesToAdd, nil
}

func (l *loadbalancers) EnsureFWRuleDeleted(lb *godo.LoadBalancer) ([]firewallRequest, error) {
	var rulesToDelete []firewallRequest

	for _, dropletID := range lb.DropletIDs {

		firewalls, _, err := l.client.Firewalls.ListByDroplet(context.TODO(), dropletID, nil)

		if err != nil {
			return nil, fmt.Errorf("Error listing firewalls for droplet %d", dropletID)
		}

		if len(firewalls) == 0 {
			continue
		}

		for _, lbRule := range lb.ForwardingRules {

			if lbfwRuleRequiresDeletion(lb.ID, lbRule, firewalls) == true &&
				requestExists(rulesToDelete, firewalls[0].ID, lb.ID, lbRule.TargetPort) == false {

				rulesToDelete = append(rulesToDelete, firewallRequest{firewalls[0].ID, createRuleRequest(lb.ID, lbRule.TargetPort)})

			}
		}

	}

	err := deleteRules(l.client, rulesToDelete)
	if err != nil {
		return rulesToDelete, fmt.Errorf("Error deleting firewall rules")
	}

	return rulesToDelete, nil
}
