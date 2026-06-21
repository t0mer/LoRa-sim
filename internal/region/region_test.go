package region

import "testing"

func TestStationNameMapping(t *testing.T) {
	cases := map[RfRegion]string{
		EU868:   "EU863-870",
		US915:   "US902-928",
		AS923_1: "AS923",
		IN865:   "IN865-867",
	}
	for r, want := range cases {
		if got := r.StationName(); got != want {
			t.Errorf("%s.StationName() = %q, want %q", r, got, want)
		}
	}
}

func TestTiers(t *testing.T) {
	tier1 := []RfRegion{EU868, US915, AU915, AS923_1}
	for _, r := range tier1 {
		if r.Tier() != 1 {
			t.Errorf("%s tier = %d, want 1", r, r.Tier())
		}
	}
	if AS923_2.Tier() != 2 || CN470.Tier() != 2 {
		t.Errorf("expected tier-2 regions")
	}
}

func TestParse(t *testing.T) {
	if r, err := Parse("eu868"); err != nil || r != EU868 {
		t.Errorf("Parse(eu868) = (%v, %v)", r, err)
	}
	if _, err := Parse("XX999"); err == nil {
		t.Error("Parse(XX999) = nil error, want error")
	}
}

func TestAllRegionsValidAndNamed(t *testing.T) {
	all := All()
	if len(all) != 13 {
		t.Errorf("All() = %d regions, want 13", len(all))
	}
	for _, r := range all {
		if !r.Valid() {
			t.Errorf("%s not valid", r)
		}
		if r.StationName() == "" {
			t.Errorf("%s has no station name", r)
		}
	}
}

func TestEndpoints(t *testing.T) {
	cups, lns := Endpoints("abcdef", "eu-west-1")
	if cups != "https://abcdef.cups.lorawan.eu-west-1.amazonaws.com:443" {
		t.Errorf("cups = %q", cups)
	}
	if lns != "wss://abcdef.lns.lorawan.eu-west-1.amazonaws.com:443" {
		t.Errorf("lns = %q", lns)
	}
}
