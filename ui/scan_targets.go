package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/doitintl/terminator/pkg/types"
)

func normalizeIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	ids := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	return ids
}

func uniqueOrderedVPCIDs(nats []types.NATGateway) []string {
	seen := make(map[string]struct{}, len(nats))
	vpcIDs := make([]string, 0)
	for _, nat := range nats {
		if _, ok := seen[nat.VPCID]; ok {
			continue
		}
		seen[nat.VPCID] = struct{}{}
		vpcIDs = append(vpcIDs, nat.VPCID)
	}
	return vpcIDs
}

func groupNATsByVPC(nats []types.NATGateway) map[string][]types.NATGateway {
	grouped := make(map[string][]types.NATGateway)
	for _, nat := range nats {
		grouped[nat.VPCID] = append(grouped[nat.VPCID], nat)
	}
	return grouped
}

func countNATsByVPC(nats []types.NATGateway) map[string]int {
	counts := make(map[string]int)
	for _, nat := range nats {
		counts[nat.VPCID]++
	}
	return counts
}

func filterNATGateways(nats []types.NATGateway, vpcIDs, natIDs []string) ([]types.NATGateway, error) {
	vpcIDs = normalizeIDs(vpcIDs)
	natIDs = normalizeIDs(natIDs)

	if len(vpcIDs) == 0 && len(natIDs) == 0 {
		return append([]types.NATGateway(nil), nats...), nil
	}

	vpcSet := make(map[string]struct{}, len(vpcIDs))
	for _, id := range vpcIDs {
		vpcSet[id] = struct{}{}
	}
	natSet := make(map[string]struct{}, len(natIDs))
	for _, id := range natIDs {
		natSet[id] = struct{}{}
	}

	knownVPCs := make(map[string]struct{})
	knownNATs := make(map[string]struct{})
	for _, nat := range nats {
		knownVPCs[nat.VPCID] = struct{}{}
		knownNATs[nat.ID] = struct{}{}
	}

	var unknownVPCIDs []string
	for _, id := range vpcIDs {
		if _, ok := knownVPCs[id]; !ok {
			unknownVPCIDs = append(unknownVPCIDs, id)
		}
	}

	var unknownNATIDs []string
	for _, id := range natIDs {
		if _, ok := knownNATs[id]; !ok {
			unknownNATIDs = append(unknownNATIDs, id)
		}
	}

	if len(unknownVPCIDs) > 0 || len(unknownNATIDs) > 0 {
		var parts []string
		if len(unknownVPCIDs) > 0 {
			parts = append(parts, fmt.Sprintf("unknown VPC ID(s): %s", strings.Join(unknownVPCIDs, ", ")))
		}
		if len(unknownNATIDs) > 0 {
			parts = append(parts, fmt.Sprintf("unknown NAT Gateway ID(s): %s", strings.Join(unknownNATIDs, ", ")))
		}
		return nil, fmt.Errorf("%s", strings.Join(parts, "; "))
	}

	filtered := make([]types.NATGateway, 0, len(nats))
	for _, nat := range nats {
		if len(vpcSet) > 0 {
			if _, ok := vpcSet[nat.VPCID]; !ok {
				continue
			}
		}
		if len(natSet) > 0 {
			if _, ok := natSet[nat.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, nat)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no NAT gateways found matching the selected VPC and NAT filters")
	}

	return filtered, nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
