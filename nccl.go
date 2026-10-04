package clustercontract

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// NCCLOptions supplies metadata absent from nccl-tests text output. Scope is
// the complete tested communicator (for example, fabric/rack-a), NOT one node
// inferred from a collective's aggregate result. These values are assertions.
type NCCLOptions struct {
	Scope                  string
	EnvironmentFingerprint string
	RecordedAt             time.Time
	Reference              string
	Description            string
}

// ImportNCCL reads the nccl-tests v2.13.10 13-column all_reduce_perf text format.
// It keeps size, datatype, operation and placement separate. Each printed row
// contributes one sample, not the benchmark's number of internal iterations.
// It does not run NCCL or verify that the supplied log came from all_reduce_perf.
func ImportNCCL(r io.Reader, opts NCCLOptions) (EvidenceBundle, error) {
	for _, f := range [][2]string{{"scope", opts.Scope}, {"environment_fingerprint", opts.EnvironmentFingerprint}, {"reference", opts.Reference}} {
		if err := nonempty(f[0], f[1]); err != nil {
			return EvidenceBundle{}, err
		}
	}
	if !validTime(opts.RecordedAt) {
		return EvidenceBundle{}, fmt.Errorf("recorded_at must be a nonzero RFC3339 timestamp")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return EvidenceBundle{}, err
	}
	if len(data) > MaxInputBytes {
		return EvidenceBundle{}, fmt.Errorf("NCCL input exceeds %d bytes", MaxInputBytes)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	byMetric := make(map[string]*Evidence)
	unknownCorrectness := make(map[string]bool)
	header, placements, units, summary := false, false, false, false
	var summaryErrors uint64
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		bad := func(message string) (EvidenceBundle, error) {
			return EvidenceBundle{}, fmt.Errorf("NCCL line %d: %s", lineNo, message)
		}
		if strings.HasPrefix(line, "#") {
			body := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			fields := strings.Fields(body)
			if strings.Contains(body, "out-of-place") || strings.Contains(body, "in-place") {
				if strings.Join(fields, " ") != "out-of-place in-place" {
					return bad("unsupported placement header; require out-of-place followed by in-place")
				}
				if placements {
					return bad("multiple benchmark headers are not supported")
				}
				placements = true
			}
			if len(fields) > 0 && fields[0] == "size" {
				want := "size count type redop root time algbw busbw #wrong time algbw busbw #wrong"
				if strings.Join(fields, " ") != want {
					return bad("unsupported column header; expected standard time/algbw/busbw/#wrong columns")
				}
				if header {
					return bad("multiple column headers are not supported")
				}
				header = true
			}
			if len(fields) > 0 && fields[0] == "(B)" {
				if strings.Join(fields, " ") != "(B) (elements) (us) (GB/s) (GB/s) (us) (GB/s) (GB/s)" {
					return bad("unsupported units header")
				}
				if units {
					return bad("duplicate units header")
				}
				units = true
			}
			if strings.HasPrefix(body, "Out of bounds values") {
				if summary {
					return bad("duplicate correctness summary")
				}
				if len(fields) != 7 || strings.Join(fields[:5], " ") != "Out of bounds values :" {
					return bad("malformed correctness summary")
				}
				v, err := strconv.ParseUint(fields[5], 10, 64)
				if err != nil {
					return bad("invalid correctness summary count")
				}
				if v == 0 && fields[6] != "OK" || v > 0 && fields[6] != "FAILED" {
					return bad("inconsistent correctness summary status")
				}
				summaryErrors = v
				summary = true
			}
			continue
		}
		if !header || !placements || !units {
			return bad("data before required placement, column and units headers")
		}
		if summary {
			return bad("unexpected data after correctness summary")
		}
		f := strings.Fields(line)
		if len(f) != 13 {
			return bad("expected exactly 13 data columns; MPI prefixes and interleaved debug output are unsupported")
		}
		size, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil || size == 0 {
			return bad("size must be positive bytes")
		}
		count, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil || count == 0 {
			return bad("count must be positive")
		}
		width, ok := map[string]uint64{"int8": 1, "uint8": 1, "int32": 4, "uint32": 4, "int64": 8, "uint64": 8, "half": 2, "float": 4, "double": 8, "bfloat16": 2}[f[2]]
		if !ok || count > math.MaxUint64/width || count*width != size {
			return bad("unsupported datatype or inconsistent size/count")
		}
		if !map[string]bool{"sum": true, "prod": true, "min": true, "max": true, "avg": true, "mulsum": true}[f[3]] {
			return bad("unsupported reduction operation")
		}
		if f[4] != "-1" {
			return bad("all_reduce_perf requires root -1")
		}
		for _, idx := range []int{5, 6, 7, 9, 10, 11} {
			v, e := strconv.ParseFloat(f[idx], 64)
			if e != nil || !finite(v) || v < 0 {
				return bad(fmt.Sprintf("column %d must be finite and nonnegative", idx+1))
			}
		}
		for _, mode := range []struct {
			name      string
			bw, wrong int
		}{{"out_of_place", 7, 8}, {"in_place", 11, 12}} {
			metric := fmt.Sprintf("nccl.all_reduce.busbw.%s.bytes_%d.type_%s.op_%s", mode.name, size, f[2], f[3])
			e, ok := byMetric[metric]
			if !ok {
				id := sha256.Sum256([]byte(opts.Scope + "\x00" + metric + "\x00" + opts.RecordedAt.UTC().Format(time.RFC3339Nano) + "\x00" + digest))
				zero := uint64(0)
				e = &Evidence{ID: fmt.Sprintf("nccl-%x", id[:12]), RecordedAt: opts.RecordedAt.UTC(), EnvironmentFingerprint: opts.EnvironmentFingerprint, Scope: opts.Scope, Metric: metric, Unit: "GB/s", Samples: []float64{}, CorrectnessErrors: &zero, Provenance: Provenance{Source: "nccl-tests/all_reduce_perf; text adapter v1", Reference: opts.Reference, SHA256: digest}}
				byMetric[metric] = e
			}
			bw, _ := strconv.ParseFloat(f[mode.bw], 64)
			e.Samples = append(e.Samples, bw)
			if f[mode.wrong] == "N/A" {
				unknownCorrectness[metric] = true
				continue
			}
			wrong, err := strconv.ParseFloat(f[mode.wrong], 64)
			if err != nil || !finite(wrong) || wrong < 0 || wrong > 1<<53 || math.Trunc(wrong) != wrong {
				return bad("correctness count must be a nonnegative exact integer or N/A")
			}
			if math.MaxUint64-*e.CorrectnessErrors < uint64(wrong) {
				return bad("correctness count overflow")
			}
			*e.CorrectnessErrors += uint64(wrong)
		}
	}
	if err := scanner.Err(); err != nil {
		return EvidenceBundle{}, fmt.Errorf("read NCCL text: %w", err)
	}
	if !header || !placements || !units || !summary || len(byMetric) == 0 {
		return EvidenceBundle{}, fmt.Errorf("NCCL log is incomplete: placement header, columns, units, data and correctness summary are required")
	}
	for _, e := range byMetric {
		if *e.CorrectnessErrors > 0 && summaryErrors == 0 {
			return EvidenceBundle{}, fmt.Errorf("NCCL log has row correctness errors but its summary reports zero")
		}
	}
	b := EvidenceBundle{Version: Version, Description: opts.Description, Evidence: []Evidence{}}
	for metric, e := range byMetric {
		// Any known error in the run prevents a clean result for a selected size
		// or placement. Retain a count rather than replacing it with a pass.
		if summaryErrors > 0 && *e.CorrectnessErrors < summaryErrors {
			count := summaryErrors
			e.CorrectnessErrors = &count
		}
		if unknownCorrectness[metric] && *e.CorrectnessErrors == 0 {
			e.CorrectnessErrors = nil
		}
		b.Evidence = append(b.Evidence, *e)
	}
	sort.Slice(b.Evidence, func(i, j int) bool { return b.Evidence[i].Metric < b.Evidence[j].Metric })
	return b, b.Validate()
}
