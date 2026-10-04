// Package clustercontract evaluates explicit customer acceptance obligations
// against recorded evidence. It does not run benchmarks or attest provenance.
package clustercontract

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const Version = "v1"

// These local-tool limits bound cross-products and report amplification, rather
// than relying only on the serialized input byte limit.
const (
	MaxObligations        = 50000
	MaxEvidenceRecords    = 100000
	MaxTotalSamples       = 1000000
	MaxSampleEvaluations  = 2000000
	MaxReportPayloadBytes = 64 << 20
	MaxTextBytes          = 1024
)

// Contract expands each criterion into one required obligation per scope.
type Contract struct {
	Version                string      `json:"version"`
	Name                   string      `json:"name"`
	EnvironmentFingerprint string      `json:"environment_fingerprint"`
	Description            string      `json:"description,omitempty"`
	Criteria               []Criterion `json:"criteria"`
	Waivers                []Waiver    `json:"waivers,omitempty"`
}

// Criterion bounds apply to EVERY sample of EVERY scope, inclusively.
type Criterion struct {
	ID                string   `json:"id"`
	Owner             string   `json:"owner"`
	Metric            string   `json:"metric"`
	Unit              string   `json:"unit"`
	Scopes            []string `json:"scopes"`
	Minimum           *float64 `json:"minimum,omitempty"`
	Maximum           *float64 `json:"maximum,omitempty"`
	MinSamples        int      `json:"min_samples"`
	MaxAgeSeconds     int64    `json:"max_age_seconds"`
	RequireZeroErrors bool     `json:"require_zero_errors,omitempty"`
}

// Waiver is scoped to exactly one obligation, with an exclusive expiry.
type Waiver struct {
	CriterionID string    `json:"criterion_id"`
	Scope       string    `json:"scope"`
	Reason      string    `json:"reason"`
	Owner       string    `json:"owner"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type EvidenceBundle struct {
	Version     string     `json:"version"`
	Description string     `json:"description,omitempty"`
	Evidence    []Evidence `json:"evidence"`
}

// Evidence is one observation record; samples must share the same metric,
// scope, environment and measurement protocol. Provenance is asserted by its
// producer. Nil CorrectnessErrors means correctness was not reported.
type Evidence struct {
	ID                     string     `json:"id"`
	RecordedAt             time.Time  `json:"recorded_at"`
	EnvironmentFingerprint string     `json:"environment_fingerprint"`
	Scope                  string     `json:"scope"`
	Metric                 string     `json:"metric"`
	Unit                   string     `json:"unit"`
	Samples                []float64  `json:"samples"`
	CorrectnessErrors      *uint64    `json:"correctness_errors,omitempty"`
	Provenance             Provenance `json:"provenance"`
}

type Provenance struct {
	Source    string `json:"source"`
	Reference string `json:"reference"`
	SHA256    string `json:"sha256,omitempty"`
}

type Verdict string

const (
	Pass        Verdict = "pass"
	Fail        Verdict = "fail"
	Incomplete  Verdict = "incomplete"
	Conditional Verdict = "conditional"
	Waived      Verdict = "waived"
)

type Report struct {
	Version                string       `json:"version"`
	Contract               string       `json:"contract"`
	Description            string       `json:"description,omitempty"`
	EvidenceDescription    string       `json:"evidence_description,omitempty"`
	EvaluatedAt            time.Time    `json:"evaluated_at"`
	EnvironmentFingerprint string       `json:"environment_fingerprint"`
	Verdict                Verdict      `json:"verdict"`
	Obligations            []Obligation `json:"obligations"`
	UnusedEvidenceIDs      []string     `json:"unused_evidence_ids"`
}

type Obligation struct {
	CriterionID       string      `json:"criterion_id"`
	Scope             string      `json:"scope"`
	Owner             string      `json:"owner"`
	Metric            string      `json:"metric"`
	Unit              string      `json:"unit"`
	Minimum           *float64    `json:"minimum,omitempty"`
	Maximum           *float64    `json:"maximum,omitempty"`
	MinSamples        int         `json:"min_samples"`
	MaxAgeSeconds     int64       `json:"max_age_seconds"`
	RequireZeroErrors bool        `json:"require_zero_errors,omitempty"`
	Status            Verdict     `json:"status"`
	UnderlyingStatus  Verdict     `json:"underlying_status"`
	Reasons           []string    `json:"reasons"`
	EvidenceID        string      `json:"evidence_id,omitempty"`
	RecordedAt        *time.Time  `json:"recorded_at,omitempty"`
	SampleCount       int         `json:"sample_count"`
	ObservedMinimum   *float64    `json:"observed_minimum,omitempty"`
	ObservedMaximum   *float64    `json:"observed_maximum,omitempty"`
	CorrectnessErrors *uint64     `json:"correctness_errors,omitempty"`
	Provenance        *Provenance `json:"provenance,omitempty"`
	Waiver            *Waiver     `json:"waiver,omitempty"`
	WaiverState       string      `json:"waiver_state,omitempty"`
}

func (r Report) ExitCode() int {
	switch r.Verdict {
	case Pass:
		return 0
	case Fail, Incomplete:
		return 1
	case Conditional:
		return 3
	default:
		return 2
	}
}

func nonempty(label, value string) error {
	if len(value) > MaxTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", label, MaxTextBytes)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if value != strings.TrimSpace(value) || strings.ContainsAny(value, "\n\r\t\x00") {
		return fmt.Errorf("%s must not contain surrounding whitespace or control separators", label)
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (c Contract) Validate() error {
	if len(c.Description) > 4096 {
		return fmt.Errorf("contract description exceeds 4096 bytes")
	}
	if c.Version != Version {
		return fmt.Errorf("contract version must be %q", Version)
	}
	if err := nonempty("contract name", c.Name); err != nil {
		return err
	}
	if err := nonempty("contract environment_fingerprint", c.EnvironmentFingerprint); err != nil {
		return err
	}
	if len(c.Criteria) == 0 {
		return fmt.Errorf("contract requires at least one criterion")
	}
	ids := make(map[string]bool)
	obligations := make(map[[2]string]bool)
	obligationCount := 0
	for i, rule := range c.Criteria {
		obligationCount += len(rule.Scopes)
		if obligationCount > MaxObligations {
			return fmt.Errorf("contract exceeds %d expanded obligations", MaxObligations)
		}
		p := fmt.Sprintf("criteria[%d]", i)
		for _, f := range [][2]string{{"id", rule.ID}, {"owner", rule.Owner}, {"metric", rule.Metric}, {"unit", rule.Unit}} {
			if err := nonempty(p+"."+f[0], f[1]); err != nil {
				return err
			}
		}
		if ids[rule.ID] {
			return fmt.Errorf("duplicate criterion id %q", rule.ID)
		}
		ids[rule.ID] = true
		if rule.Minimum == nil && rule.Maximum == nil {
			return fmt.Errorf("%s requires minimum or maximum", p)
		}
		if rule.Minimum != nil && !finite(*rule.Minimum) || rule.Maximum != nil && !finite(*rule.Maximum) {
			return fmt.Errorf("%s bounds must be finite", p)
		}
		if rule.Minimum != nil && rule.Maximum != nil && *rule.Minimum > *rule.Maximum {
			return fmt.Errorf("%s minimum exceeds maximum", p)
		}
		if rule.MinSamples <= 0 {
			return fmt.Errorf("%s.min_samples must be positive", p)
		}
		if rule.MaxAgeSeconds <= 0 || rule.MaxAgeSeconds > int64(math.MaxInt64)/int64(time.Second) {
			return fmt.Errorf("%s.max_age_seconds is outside supported positive duration range", p)
		}
		if len(rule.Scopes) == 0 {
			return fmt.Errorf("%s requires at least one scope", p)
		}
		for _, scope := range rule.Scopes {
			if err := nonempty(p+" scope", scope); err != nil {
				return err
			}
			key := [2]string{rule.ID, scope}
			if obligations[key] {
				return fmt.Errorf("duplicate scope %q in criterion %q", scope, rule.ID)
			}
			obligations[key] = true
		}
	}
	waivers := make(map[[2]string]bool)
	for i, w := range c.Waivers {
		key := [2]string{w.CriterionID, w.Scope}
		if !obligations[key] {
			return fmt.Errorf("waivers[%d] references unknown criterion/scope %q/%q", i, w.CriterionID, w.Scope)
		}
		if waivers[key] {
			return fmt.Errorf("duplicate waiver for %q/%q", w.CriterionID, w.Scope)
		}
		waivers[key] = true
		if err := nonempty(fmt.Sprintf("waivers[%d].owner", i), w.Owner); err != nil {
			return err
		}
		if err := nonempty(fmt.Sprintf("waivers[%d].reason", i), w.Reason); err != nil {
			return err
		}
		if !validTime(w.ExpiresAt) {
			return fmt.Errorf("waivers[%d].expires_at must be a nonzero RFC3339 timestamp", i)
		}
	}
	return nil
}

func validTime(t time.Time) bool { return !t.IsZero() && t.Year() >= 1 && t.Year() <= 9999 }

func (b EvidenceBundle) Validate() error {
	if len(b.Description) > 4096 {
		return fmt.Errorf("evidence description exceeds 4096 bytes")
	}
	if len(b.Evidence) > MaxEvidenceRecords {
		return fmt.Errorf("evidence exceeds %d records", MaxEvidenceRecords)
	}
	if b.Version != Version {
		return fmt.Errorf("evidence version must be %q", Version)
	}
	if b.Evidence == nil {
		return fmt.Errorf("evidence must be an array (use [] for no observations)")
	}
	ids := make(map[string]bool)
	totalSamples := 0
	timestamps := make(map[struct{ Scope, Metric, At string }]bool)
	for i, e := range b.Evidence {
		totalSamples += len(e.Samples)
		if totalSamples > MaxTotalSamples {
			return fmt.Errorf("evidence exceeds %d total samples", MaxTotalSamples)
		}
		p := fmt.Sprintf("evidence[%d]", i)
		for _, f := range [][2]string{{"id", e.ID}, {"scope", e.Scope}, {"metric", e.Metric}, {"unit", e.Unit}, {"environment_fingerprint", e.EnvironmentFingerprint}, {"provenance.source", e.Provenance.Source}, {"provenance.reference", e.Provenance.Reference}} {
			if err := nonempty(p+"."+f[0], f[1]); err != nil {
				return err
			}
		}
		if ids[e.ID] {
			return fmt.Errorf("duplicate evidence id %q", e.ID)
		}
		ids[e.ID] = true
		if !validTime(e.RecordedAt) {
			return fmt.Errorf("%s.recorded_at must be a nonzero RFC3339 timestamp", p)
		}
		key := struct{ Scope, Metric, At string }{e.Scope, e.Metric, e.RecordedAt.UTC().Format(time.RFC3339Nano)}
		if timestamps[key] {
			return fmt.Errorf("ambiguous evidence: same scope/metric/time at %s", p)
		}
		timestamps[key] = true
		if len(e.Samples) == 0 {
			return fmt.Errorf("%s.samples must contain at least one sample", p)
		}
		for j, v := range e.Samples {
			if !finite(v) {
				return fmt.Errorf("%s.samples[%d] must be finite", p, j)
			}
		}
		if e.Provenance.SHA256 != "" {
			if len(e.Provenance.SHA256) != 64 || strings.IndexFunc(e.Provenance.SHA256, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) >= 0 {
				return fmt.Errorf("%s.provenance.sha256 must be 64 lowercase hex characters", p)
			}
		}
	}
	return nil
}

// Evaluate selects the newest record for each (scope, metric), then checks its
// compatibility. It never falls back to an older passing record. Callers must
// provide evaluation time explicitly; no wall clock or network is consulted.
func Evaluate(c Contract, b EvidenceBundle, at time.Time) (Report, error) {
	if err := c.Validate(); err != nil {
		return Report{}, err
	}
	if err := b.Validate(); err != nil {
		return Report{}, err
	}
	if !validTime(at) {
		return Report{}, fmt.Errorf("evaluation time must be a nonzero RFC3339 timestamp")
	}
	at = at.UTC()
	r := Report{Version: Version, Contract: c.Name, Description: c.Description, EvidenceDescription: b.Description, EvaluatedAt: at, EnvironmentFingerprint: c.EnvironmentFingerprint, Verdict: Pass, Obligations: []Obligation{}, UnusedEvidenceIDs: []string{}}
	latest := make(map[[2]string]Evidence)
	for _, e := range b.Evidence {
		k := [2]string{e.Scope, e.Metric}
		old, ok := latest[k]
		if !ok || e.RecordedAt.After(old.RecordedAt) {
			latest[k] = e
		}
	}
	waivers := make(map[[2]string]Waiver)
	for _, w := range c.Waivers {
		w.ExpiresAt = w.ExpiresAt.UTC()
		waivers[[2]string{w.CriterionID, w.Scope}] = w
	}
	used := make(map[string]bool)
	sampleWork, reportPayload := 0, 0
	for _, rule := range c.Criteria {
		for _, scope := range rule.Scopes {
			o := Obligation{CriterionID: rule.ID, Scope: scope, Owner: rule.Owner, Metric: rule.Metric, Unit: rule.Unit, Minimum: rule.Minimum, Maximum: rule.Maximum, MinSamples: rule.MinSamples, MaxAgeSeconds: rule.MaxAgeSeconds, RequireZeroErrors: rule.RequireZeroErrors, Status: Pass, Reasons: []string{}}
			e, ok := latest[[2]string{scope, rule.Metric}]
			if ok {
				sampleWork += len(e.Samples)
				if sampleWork > MaxSampleEvaluations {
					return Report{}, fmt.Errorf("evaluation exceeds %d sample comparisons; narrow the contract or evidence bundle", MaxSampleEvaluations)
				}
			}
			if !ok {
				o.Status = Incomplete
				o.Reasons = append(o.Reasons, "missing evidence")
			} else {
				used[e.ID] = true
				o.EvidenceID = e.ID
				et := e.RecordedAt.UTC()
				o.RecordedAt = &et
				o.SampleCount = len(e.Samples)
				p := e.Provenance
				o.Provenance = &p
				o.CorrectnessErrors = e.CorrectnessErrors
				lo, hi := e.Samples[0], e.Samples[0]
				for _, v := range e.Samples {
					lo = math.Min(lo, v)
					hi = math.Max(hi, v)
				}
				o.ObservedMinimum = &lo
				o.ObservedMaximum = &hi
				if e.EnvironmentFingerprint != c.EnvironmentFingerprint {
					o.Reasons = append(o.Reasons, "environment fingerprint mismatch")
				}
				if e.Unit != rule.Unit {
					o.Reasons = append(o.Reasons, "unit mismatch: observed "+e.Unit)
				}
				if e.RecordedAt.After(at) {
					o.Reasons = append(o.Reasons, "evidence timestamp is after evaluation time")
				} else if at.Sub(e.RecordedAt) > time.Duration(rule.MaxAgeSeconds)*time.Second {
					o.Reasons = append(o.Reasons, "stale evidence")
				}
				if len(e.Samples) < rule.MinSamples {
					o.Reasons = append(o.Reasons, fmt.Sprintf("insufficient samples: got %d, require %d", len(e.Samples), rule.MinSamples))
				}
				if rule.RequireZeroErrors && e.CorrectnessErrors == nil {
					o.Reasons = append(o.Reasons, "correctness result not reported")
				}
				if len(o.Reasons) > 0 {
					o.Status = Incomplete
				} else {
					if rule.Minimum != nil && lo < *rule.Minimum {
						o.Reasons = append(o.Reasons, fmt.Sprintf("sample minimum %g is below required %g", lo, *rule.Minimum))
					}
					if rule.Maximum != nil && hi > *rule.Maximum {
						o.Reasons = append(o.Reasons, fmt.Sprintf("sample maximum %g is above required %g", hi, *rule.Maximum))
					}
					if len(o.Reasons) > 0 {
						o.Status = Fail
					}
				}
				// A known correctness failure is a failure even if coverage is also incomplete.
				if e.CorrectnessErrors != nil && *e.CorrectnessErrors > 0 {
					o.Status = Fail
					o.Reasons = append(o.Reasons, fmt.Sprintf("%d correctness errors reported", *e.CorrectnessErrors))
				}
			}
			o.UnderlyingStatus = o.Status
			if w, ok := waivers[[2]string{rule.ID, scope}]; ok {
				o.Waiver = &w
				if !at.Before(w.ExpiresAt) {
					o.WaiverState = "expired"
				} else if o.Status == Pass {
					o.WaiverState = "not_needed"
				} else {
					o.WaiverState = "applied"
					o.Status = Waived
				}
			}
			// Charge the complete serialized obligation, including repeated
			// provenance and waiver text, before retaining it in the report.
			payload, err := json.Marshal(o)
			if err != nil {
				return Report{}, fmt.Errorf("encode obligation: %w", err)
			}
			reportPayload += len(payload)
			if reportPayload > MaxReportPayloadBytes {
				return Report{}, fmt.Errorf("report obligation payload exceeds %d bytes; narrow the contract", MaxReportPayloadBytes)
			}
			r.Obligations = append(r.Obligations, o)
		}
	}
	sort.Slice(r.Obligations, func(i, j int) bool {
		a, b := r.Obligations[i], r.Obligations[j]
		if a.CriterionID != b.CriterionID {
			return a.CriterionID < b.CriterionID
		}
		return a.Scope < b.Scope
	})
	for _, o := range r.Obligations {
		switch o.Status {
		case Fail:
			r.Verdict = Fail
		case Incomplete:
			if r.Verdict != Fail {
				r.Verdict = Incomplete
			}
		case Waived:
			if r.Verdict == Pass {
				r.Verdict = Conditional
			}
		}
	}
	for _, e := range b.Evidence {
		if !used[e.ID] {
			r.UnusedEvidenceIDs = append(r.UnusedEvidenceIDs, e.ID)
		}
	}
	sort.Strings(r.UnusedEvidenceIDs)
	return r, nil
}
