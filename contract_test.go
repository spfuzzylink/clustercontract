package clustercontract

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testAt = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }
func up(v uint64) *uint64   { return &v }
func fixture() (Contract, EvidenceBundle) {
	c := Contract{Version: Version, Name: "SYNTHETIC NOT GPU VALIDATION", EnvironmentFingerprint: "synthetic-env", Criteria: []Criterion{{ID: "bandwidth", Owner: "acceptance-owner", Metric: "read", Unit: "GB/s", Scopes: []string{"node-b", "node-a"}, Minimum: fp(100), MinSamples: 2, MaxAgeSeconds: 3600, RequireZeroErrors: true}}}
	b := EvidenceBundle{Version: Version, Evidence: []Evidence{}}
	for _, s := range []string{"node-a", "node-b"} {
		b.Evidence = append(b.Evidence, Evidence{ID: s, Scope: s, Metric: "read", Unit: "GB/s", Samples: []float64{110, 120}, RecordedAt: testAt.Add(-time.Minute), EnvironmentFingerprint: "synthetic-env", CorrectnessErrors: up(0), Provenance: Provenance{Source: "synthetic", Reference: "test"}})
	}
	return c, b
}

func TestAcceptanceNeverPoolsOrHidesMissingEvidence(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Contract, *EvidenceBundle)
		want Verdict
	}{
		{"pass", func(*Contract, *EvidenceBundle) {}, Pass},
		{"worst node despite healthy average", func(c *Contract, b *EvidenceBundle) {
			b.Evidence[0].Samples = []float64{1000, 1000}
			b.Evidence[1].Samples = []float64{99, 99}
		}, Fail},
		{"worst sample despite healthy average", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Samples = []float64{99, 1000} }, Fail},
		{"missing node", func(c *Contract, b *EvidenceBundle) { b.Evidence = b.Evidence[:1] }, Incomplete},
		{"unknown scope cannot satisfy node", func(c *Contract, b *EvidenceBundle) { b.Evidence[1].Scope = "node-c" }, Incomplete},
		{"missing all", func(c *Contract, b *EvidenceBundle) { b.Evidence = []Evidence{} }, Incomplete},
		{"stale", func(c *Contract, b *EvidenceBundle) {
			b.Evidence[0].RecordedAt = testAt.Add(-time.Hour - time.Nanosecond)
		}, Incomplete},
		{"fresh boundary inclusive", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].RecordedAt = testAt.Add(-time.Hour) }, Pass},
		{"future", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].RecordedAt = testAt.Add(time.Nanosecond) }, Incomplete},
		{"environment", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].EnvironmentFingerprint = "other" }, Incomplete},
		{"unit", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Unit = "GiB/s" }, Incomplete},
		{"sample count", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Samples = []float64{110} }, Incomplete},
		{"reported correctness failure", func(c *Contract, b *EvidenceBundle) {
			c.Criteria[0].RequireZeroErrors = false
			b.Evidence[0].CorrectnessErrors = up(1)
		}, Fail},
		{"unknown correctness", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].CorrectnessErrors = nil }, Incomplete},
		{"inclusive threshold", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Samples = []float64{100, 100} }, Pass},
		{"maximum bound", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].Maximum = fp(115) }, Fail},
		{"latest failed observation", func(c *Contract, b *EvidenceBundle) {
			e := b.Evidence[0]
			e.ID = "regression"
			e.RecordedAt = testAt
			e.Samples = []float64{50, 60}
			b.Evidence = append(b.Evidence, e)
		}, Fail},
		{"latest incompatible prevents fallback", func(c *Contract, b *EvidenceBundle) {
			e := b.Evidence[0]
			e.ID = "new-env"
			e.RecordedAt = testAt
			e.EnvironmentFingerprint = "other"
			b.Evidence = append(b.Evidence, e)
		}, Incomplete},
		{"samples not pooled across records", func(c *Contract, b *EvidenceBundle) {
			e := b.Evidence[0]
			e.ID = "new"
			e.RecordedAt = testAt
			e.Samples = []float64{110}
			b.Evidence = append(b.Evidence, e)
		}, Incomplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, b := fixture()
			tt.edit(&c, &b)
			r, err := Evaluate(c, b, testAt)
			if err != nil {
				t.Fatal(err)
			}
			if r.Verdict != tt.want {
				t.Fatalf("want %s got %s: %+v", tt.want, r.Verdict, r.Obligations)
			}
		})
	}
}

func TestWaiversAreConditionalAndExpire(t *testing.T) {
	c, b := fixture()
	b.Evidence = b.Evidence[:1]
	c.Waivers = []Waiver{{CriterionID: "bandwidth", Scope: "node-b", Owner: "customer", Reason: "replacement node pending", ExpiresAt: testAt.Add(time.Hour)}}
	r, err := Evaluate(c, b, testAt)
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != Conditional || r.ExitCode() != 3 || r.Obligations[1].UnderlyingStatus != Incomplete || r.Obligations[1].WaiverState != "applied" {
		t.Fatalf("waiver hidden: %+v", r)
	}
	r, err = Evaluate(c, b, testAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != Incomplete || r.Obligations[1].WaiverState != "expired" {
		t.Fatalf("expiry accepted: %+v", r)
	}
	c, b = fixture()
	c.Waivers = []Waiver{{CriterionID: "bandwidth", Scope: "node-b", Owner: "customer", Reason: "conditional", ExpiresAt: testAt.Add(time.Hour)}}
	b.Evidence[0].Samples = []float64{1, 2}
	b.Evidence[1].Samples = []float64{1, 2}
	r, err = Evaluate(c, b, testAt)
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != Fail {
		t.Fatal("waiver for one scope hid unwaived failure")
	}
}

func TestInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Contract, *EvidenceBundle)
	}{
		{"duplicate criterion", func(c *Contract, b *EvidenceBundle) { c.Criteria = append(c.Criteria, c.Criteria[0]) }},
		{"duplicate scope", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].Scopes = []string{"node-a", "node-a"} }},
		{"duplicate evidence id", func(c *Contract, b *EvidenceBundle) { b.Evidence[1].ID = b.Evidence[0].ID }},
		{"ambiguous timestamp", func(c *Contract, b *EvidenceBundle) {
			e := b.Evidence[0]
			e.ID = "ambiguous"
			b.Evidence = append(b.Evidence, e)
		}},
		{"no criteria", func(c *Contract, b *EvidenceBundle) { c.Criteria = nil }},
		{"no bound", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].Minimum = nil }},
		{"bounds reversed", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].Maximum = fp(99) }},
		{"NaN sample", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Samples[0] = math.NaN() }},
		{"Inf bound", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].Minimum = fp(math.Inf(1)) }},
		{"zero samples", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Samples = nil }},
		{"zero minimum count", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].MinSamples = 0 }},
		{"overflow age", func(c *Contract, b *EvidenceBundle) { c.Criteria[0].MaxAgeSeconds = math.MaxInt64 }},
		{"missing provenance", func(c *Contract, b *EvidenceBundle) { b.Evidence[0].Provenance.Reference = "" }},
		{"unknown waiver", func(c *Contract, b *EvidenceBundle) { c.Waivers = []Waiver{{CriterionID: "typo", Scope: "node-a"}} }},
		{"duplicate waiver", func(c *Contract, b *EvidenceBundle) {
			w := Waiver{CriterionID: "bandwidth", Scope: "node-a", Owner: "x", Reason: "x", ExpiresAt: testAt}
			c.Waivers = []Waiver{w, w}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, b := fixture()
			tt.edit(&c, &b)
			if _, err := Evaluate(c, b, testAt); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestStrictJSON(t *testing.T) {
	c, b := fixture()
	cb, _ := json.Marshal(c)
	eb, _ := json.Marshal(b)
	for _, body := range []string{
		`{"version":"v1","version":"v1"}`,
		strings.Replace(string(cb), `"version"`, `"VERSION"`, 1),
		strings.Replace(string(cb), `"min_samples":2`, `"min_samples":2,"typo":0`, 1),
		strings.Replace(string(cb), `"min_samples":2`, `"min_samples":null`, 1),
		string(cb) + ` {}`, "null", `[]`, string(cb[:len(cb)-1]),
	} {
		if _, err := DecodeContract(strings.NewReader(body)); err == nil {
			t.Fatalf("accepted invalid JSON %s", body)
		}
	}
	if _, err := DecodeContract(bytes.NewReader(cb)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeEvidence(bytes.NewReader(eb)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeEvidence(strings.NewReader(strings.Replace(string(eb), `[110,120]`, `[110,null]`, 1))); err == nil {
		t.Fatal("null silently became a numeric zero")
	}
}

func TestDeterministicReports(t *testing.T) {
	c, b := fixture()
	r1, err := Evaluate(c, b, testAt)
	if err != nil {
		t.Fatal(err)
	}
	c.Criteria[0].Scopes = []string{"node-a", "node-b"}
	b.Evidence[0], b.Evidence[1] = b.Evidence[1], b.Evidence[0]
	r2, err := Evaluate(c, b, testAt.In(time.FixedZone("test", 3600)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("input ordering/timezone changed report")
	}
	var out bytes.Buffer
	if err := WriteMarkdown(&out, r1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "SYNTHETIC NOT GPU VALIDATION") || !strings.Contains(out.String(), "node-a") {
		t.Fatal("incomplete markdown")
	}
}

func FuzzDecodeContract(f *testing.F) {
	c, _ := fixture()
	data, _ := json.Marshal(c)
	f.Add(data)
	f.Add([]byte(`{"version":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := DecodeContract(bytes.NewReader(data))
		if err == nil {
			if err := c.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestExpandedWorkAndTextLimits(t *testing.T) {
	c, b := fixture()
	c.Criteria[0].Scopes = make([]string, MaxObligations+1)
	if _, err := Evaluate(c, b, testAt); err == nil || !strings.Contains(err.Error(), "expanded obligations") {
		t.Fatalf("obligation expansion accepted: %v", err)
	}
	c, b = fixture()
	b.Evidence[0].Samples = make([]float64, MaxTotalSamples+1)
	if _, err := Evaluate(c, b, testAt); err == nil || !strings.Contains(err.Error(), "total samples") {
		t.Fatalf("sample expansion accepted: %v", err)
	}
	c, b = fixture()
	b.Evidence[0].Provenance.Reference = strings.Repeat("a", MaxTextBytes+1)
	if _, err := Evaluate(c, b, testAt); err == nil {
		t.Fatal("unbounded metadata accepted")
	}
	c, b = fixture()
	c.Criteria[0].Scopes = []string{"node-a"}
	b.Evidence = b.Evidence[:1]
	b.Evidence[0].Samples = make([]float64, MaxTotalSamples)
	for i := 1; i < 3; i++ {
		rule := c.Criteria[0]
		rule.ID = string(rune('a' + i))
		c.Criteria = append(c.Criteria, rule)
	}
	if _, err := Evaluate(c, b, testAt); err == nil || !strings.Contains(err.Error(), "sample comparisons") {
		t.Fatalf("cross-product work accepted: %v", err)
	}
}
