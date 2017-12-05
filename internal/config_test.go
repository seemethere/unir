package internal

import (
	"testing"
)

func TestReadConfig(t *testing.T) {
	input := []byte(`
whitelist:
- bob
- alice
approvals_needed: 3
consensus_needed: false
`)
	conf, err := ReadConfig(input)
	if err != nil {
		t.Errorf("Error reading in configuration: %v", err)
		return
	}
	if conf.ApprovalsNeeded != 3 {
		t.Errorf("Expected 3 ApprovalsNeeded, got %v instead", conf.ApprovalsNeeded)
		return
	}
	if conf.ConsensusNeeded {
		t.Errorf("Expected false ConsensusNeeded, got true instead")
		return
	}
	userMap := make(map[string]bool)
	for _, user := range conf.Whitelist {
		userMap[user] = true
	}
	for _, expectedUser := range []string{"bob", "alice"} {
		found := userMap[expectedUser]
		if !found {
			t.Errorf("Expected to find user %s but was not found...", expectedUser)
		}
	}
}
