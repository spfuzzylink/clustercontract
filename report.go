package clustercontract

import (
	"fmt"
	"html"
	"io"
	"strings"
	"time"
)

// WriteMarkdown emits a deterministic, reviewable report, including every
// waived obligation and its underlying outcome. It performs no file access.
func WriteMarkdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# ClusterContract: %s\n\n**Verdict: %s**  \nEvaluated at: %s  \nEnvironment: %s\n\n", md(r.Contract), r.Verdict, r.EvaluatedAt.Format(time.RFC3339Nano), md(r.EnvironmentFingerprint))
	if r.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", md(r.Description))
	}
	if r.EvidenceDescription != "" {
		fmt.Fprintf(&b, "Evidence: %s\n\n", md(r.EvidenceDescription))
	}
	b.WriteString("| Criterion | Scope | Owner | Status | Underlying | Samples | Observed range | Unit | Reasons |\n|---|---|---|---|---|---:|---|---|---|\n")
	for _, o := range r.Obligations {
		observed := "unavailable"
		if o.ObservedMinimum != nil {
			observed = fmt.Sprintf("%g … %g", *o.ObservedMinimum, *o.ObservedMaximum)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d | %s | %s | %s |\n", md(o.CriterionID), md(o.Scope), md(o.Owner), o.Status, o.UnderlyingStatus, o.SampleCount, observed, md(o.Unit), md(strings.Join(o.Reasons, "; ")))
	}
	b.WriteString("\n## Obligation details\n\n")
	for _, o := range r.Obligations {
		fmt.Fprintf(&b, "### %s / %s\n\nMetric: %s; samples required: %d; maximum age: %d seconds; correctness required: %t.\n\n", md(o.CriterionID), md(o.Scope), md(o.Metric), o.MinSamples, o.MaxAgeSeconds, o.RequireZeroErrors)
		if o.Minimum != nil {
			fmt.Fprintf(&b, "Minimum: %g %s.  \n", *o.Minimum, md(o.Unit))
		}
		if o.Maximum != nil {
			fmt.Fprintf(&b, "Maximum: %g %s.  \n", *o.Maximum, md(o.Unit))
		}
		b.WriteString("\n")
		if o.EvidenceID != "" {
			fmt.Fprintf(&b, "Evidence: %s, recorded %s.  \nSource: %s. Reference: %s.\n\n", md(o.EvidenceID), o.RecordedAt.Format(time.RFC3339Nano), md(o.Provenance.Source), md(o.Provenance.Reference))
			if o.Provenance.SHA256 != "" {
				fmt.Fprintf(&b, "SHA-256: %s\n\n", o.Provenance.SHA256)
			}
		}
		if o.Waiver != nil {
			fmt.Fprintf(&b, "**Waiver: %s**; owner: %s; expires: %s. Reason: %s.\n\n", o.WaiverState, md(o.Waiver.Owner), o.Waiver.ExpiresAt.Format(time.RFC3339Nano), md(o.Waiver.Reason))
		}
	}
	if len(r.UnusedEvidenceIDs) > 0 {
		fmt.Fprintf(&b, "Unused evidence records: %s.\n\n", md(strings.Join(r.UnusedEvidenceIDs, ", ")))
	}
	b.WriteString("This verdict evaluates supplied evidence and metadata. It does not attest hardware, benchmark execution, or provenance authenticity.\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func md(s string) string {
	s = html.EscapeString(s)
	r := strings.NewReplacer("\\", "\\\\", "|", "\\|", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`", "#", "\\#", "\r", " ", "\n", " ")
	return r.Replace(s)
}
