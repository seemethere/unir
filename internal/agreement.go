package internal

import (
	"fmt"
	"strings"
)

type AgreementOptions struct {
	Threshold      int
	NeedsConsensus bool
}

// AgreementReached determines whether members have reached an agreement
// If the number of members who answered true is greater than the
// threshold the function returns true.
func AgreementReached(members []string, votes map[string]bool, opts *AgreementOptions) (bool, string) {
	if opts == nil {
		opts = &AgreementOptions{Threshold: 1, NeedsConsensus: true}
	}
	numFor := 0
	for _, member := range members {
		isFor, voted := votes[strings.ToLower(member)]
		// cases where members do not have a vote in votes
		if !voted {
			continue
		}
		if isFor {
			numFor++
		} else if (*opts).NeedsConsensus {
			// Return out early if we need a consensus
			return false, "consensus needed"
		}
	}
	if numFor >= (*opts).Threshold {
		return true, ""
	} else {
		return false, fmt.Sprintf("%d more approval(s) needed", (*opts).Threshold-numFor)
	}
}
