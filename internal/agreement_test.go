package internal

import "testing"

func TestAgreementReached(t *testing.T) {
	testcases := []struct {
		members  []string
		votes    map[string]bool
		opts     *AgreementOptions
		expected bool
	}{
		{ // simple consensus
			[]string{"foo", "bar"},
			map[string]bool{"foo": true, "bar": true, "baz": false},
			nil,
			true,
		},
		{ // simple non-consensus
			[]string{"foo", "bar"},
			map[string]bool{"foo": true, "bar": false, "baz": false},
			nil,
			false,
		},
		{ // simple threshold reached
			[]string{"foo", "bar", "baz"},
			map[string]bool{"foo": true, "bar": true, "baz": false},
			&AgreementOptions{Threshold: 2, NeedsConsensus: false},
			true,
		},
		{ //simple threshold not reached
			[]string{"foo", "bar", "baz"},
			map[string]bool{"foo": true, "bar": true, "baz": false},
			&AgreementOptions{Threshold: 3, NeedsConsensus: false},
			false,
		},
	}

	for _, testcase := range testcases {
		actual := AgreementReached(testcase.members, testcase.votes, testcase.opts)
		if actual != testcase.expected {
			t.Errorf(
				"Expected: %v, Got %v\nmembers: %v\nvotes: %v\nopts: %v",
				testcase.expected,
				actual,
				testcase.members,
				testcase.votes,
				*testcase.opts,
			)
		}
	}
}