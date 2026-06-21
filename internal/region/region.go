// Package region models the AWS IoT Core for LoRaWAN RfRegion values and maps
// them to LoRa Basics Station region names and AWS endpoints. Tier-1 regions are
// validated against live AWS gateway onboarding; Tier-2 are modeled.
package region

import (
	"fmt"
	"strings"
)

// RfRegion is an AWS RfRegion enum value.
type RfRegion string

const (
	EU868   RfRegion = "EU868"
	US915   RfRegion = "US915"
	AU915   RfRegion = "AU915"
	AS923_1 RfRegion = "AS923-1"
	AS923_2 RfRegion = "AS923-2"
	AS923_3 RfRegion = "AS923-3"
	AS923_4 RfRegion = "AS923-4"
	EU433   RfRegion = "EU433"
	CN470   RfRegion = "CN470"
	CN779   RfRegion = "CN779"
	RU864   RfRegion = "RU864"
	KR920   RfRegion = "KR920"
	IN865   RfRegion = "IN865"
)

type info struct {
	stationName string
	tier        int
}

// table maps each RfRegion to its Basic Station region name and validation tier.
var table = map[RfRegion]info{
	EU868:   {"EU863-870", 1},
	US915:   {"US902-928", 1},
	AU915:   {"AU915-928", 1},
	AS923_1: {"AS923", 1},
	AS923_2: {"AS923-2", 2},
	AS923_3: {"AS923-3", 2},
	AS923_4: {"AS923-4", 2},
	EU433:   {"EU433", 2},
	CN470:   {"CN470-510", 2},
	CN779:   {"CN779-787", 2},
	RU864:   {"RU864-870", 2},
	KR920:   {"KR920-923", 2},
	IN865:   {"IN865-867", 2},
}

// All returns every supported RfRegion (stable order).
func All() []RfRegion {
	return []RfRegion{
		EU868, US915, AU915, AS923_1, AS923_2, AS923_3, AS923_4,
		EU433, CN470, CN779, RU864, KR920, IN865,
	}
}

// Parse validates and normalizes a region string (case-insensitive).
func Parse(s string) (RfRegion, error) {
	up := strings.ToUpper(strings.TrimSpace(s))
	for r := range table {
		if strings.ToUpper(string(r)) == up {
			return r, nil
		}
	}
	return "", fmt.Errorf("unknown RfRegion %q", s)
}

// Valid reports whether r is a known region.
func (r RfRegion) Valid() bool {
	_, ok := table[r]
	return ok
}

// StationName returns the LoRa Basics Station region name (as advertised in
// router_config), e.g. EU868 -> "EU863-870".
func (r RfRegion) StationName() string {
	return table[r].stationName
}

// Tier returns 1 (validated against live AWS) or 2 (modeled).
func (r RfRegion) Tier() int {
	return table[r].tier
}

// Endpoints derives the AWS IoT Core for LoRaWAN CUPS and LNS endpoint URLs for
// an account prefix and AWS region (e.g. "eu-west-1"). The credential files
// normally carry these URIs; this helper documents/derives them when needed.
func Endpoints(prefix, awsRegion string) (cupsURI, lnsURI string) {
	cupsURI = fmt.Sprintf("https://%s.cups.lorawan.%s.amazonaws.com:443", prefix, awsRegion)
	lnsURI = fmt.Sprintf("wss://%s.lns.lorawan.%s.amazonaws.com:443", prefix, awsRegion)
	return cupsURI, lnsURI
}
