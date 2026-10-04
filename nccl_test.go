package clustercontract

import (
	"strings"
	"testing"
)

// Invented values in the published nccl-tests column layout. Not GPU results.
const syntheticNCCL = `# SYNTHETIC NOT GPU VALIDATION
# out-of-place in-place
# size count type redop root time algbw busbw #wrong time algbw busbw #wrong
# (B) (elements) (us) (GB/s) (GB/s) (us) (GB/s) (GB/s)
1048576 262144 float sum -1 12.5 80.00 140.00 0 13.2 75.00 131.00 0
1048576 262144 float sum -1 12.7 79.00 138.25 0 13.4 74.00 129.50 0
2097152 524288 float sum -1 25.0 80.00 140.00 0 26.4 75.00 131.00 0
# Out of bounds values : 0 OK
# Avg bus bandwidth : 134.96
`

func options() NCCLOptions {
	return NCCLOptions{Scope: "communicator/rack-a", EnvironmentFingerprint: "synthetic", RecordedAt: testAt, Reference: "fixture", Description: "SYNTHETIC NOT GPU VALIDATION"}
}

func TestNCCLPlacementSizeAndSampleSeparation(t *testing.T) {
	b, err := ImportNCCL(strings.NewReader(syntheticNCCL), options())
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Evidence) != 4 {
		t.Fatalf("want four independent metrics, got %d", len(b.Evidence))
	}
	seen := map[string]bool{}
	for _, e := range b.Evidence {
		if len(e.Provenance.SHA256) != 64 {
			t.Fatal("missing input digest")
		}
		if strings.Contains(e.Metric, "bytes_1048576") {
			if len(e.Samples) != 2 {
				t.Fatal("wrong repeat count")
			}
			if strings.Contains(e.Metric, "out_of_place") && e.Samples[0] != 140 {
				t.Fatal("out-of-place confused with in-place or algbw")
			}
			if strings.Contains(e.Metric, "in_place") && e.Samples[0] != 131 {
				t.Fatal("in-place confused with out-of-place")
			}
		}
		seen[e.ID] = true
	}
	if len(seen) != 4 {
		t.Fatal("nonunique evidence IDs")
	}
}

func TestNCCLCorrectness(t *testing.T) {
	for _, tt := range []struct {
		name, log string
		want      Verdict
	}{
		{"known errors", strings.Replace(syntheticNCCL, "values : 0 OK", "values : 1 FAILED", 1), Fail},
		{"correctness not checked", strings.ReplaceAll(syntheticNCCL, " 0", " N/A"), Incomplete},
	} {
		t.Run(tt.name, func(t *testing.T) {
			log := tt.log
			if tt.want == Incomplete {
				log = strings.Replace(log, "values : N/A OK", "values : 0 OK", 1)
			}
			b, err := ImportNCCL(strings.NewReader(log), options())
			if err != nil {
				t.Fatal(err)
			}
			e := b.Evidence[0]
			c := Contract{Version: Version, Name: "synthetic", EnvironmentFingerprint: e.EnvironmentFingerprint, Criteria: []Criterion{{ID: "nccl", Owner: "owner", Metric: e.Metric, Unit: e.Unit, Scopes: []string{e.Scope}, Minimum: fp(1), MinSamples: 1, MaxAgeSeconds: 3600, RequireZeroErrors: true}}}
			r, err := Evaluate(c, b, testAt)
			if err != nil {
				t.Fatal(err)
			}
			if r.Verdict != tt.want {
				t.Fatalf("got %s want %s", r.Verdict, tt.want)
			}
		})
	}
}

func TestNCCLRejectsUnsupportedAndPartialLogs(t *testing.T) {
	for _, log := range []string{
		strings.Replace(syntheticNCCL, "# out-of-place in-place", "# in-place out-of-place", 1),
		strings.Replace(syntheticNCCL, "# out-of-place in-place", "# extra out-of-place in-place", 1),
		strings.Replace(syntheticNCCL, "busbw #wrong time", "busbw errors time", 1),
		strings.Replace(syntheticNCCL, "(GB/s)", "(GiB/s)", 1),
		strings.Replace(syntheticNCCL, "# Out of bounds values : 0 OK", "", 1),
		strings.Replace(syntheticNCCL, "140.00", "NaN", 1),
		strings.Replace(syntheticNCCL, "262144", "262143", 1),
		strings.Replace(syntheticNCCL, " -1 ", " 0 ", 1),
		strings.Replace(syntheticNCCL, "140.00 0", "140.00 1", 1),
		strings.Replace(syntheticNCCL, "140.00 0", "140.00 -1", 1),
		syntheticNCCL + "truncated run error\n",
		syntheticNCCL + syntheticNCCL,
		"",
	} {
		if _, err := ImportNCCL(strings.NewReader(log), options()); err == nil {
			t.Fatalf("invalid log accepted: %s", log)
		}
	}
}
