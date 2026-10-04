package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	cc "github.com/spfuzzylink/clustercontract"
)

func TestCLIExitCodesAndReports(t *testing.T) {
	for _, tt := range []struct {
		name, contract, evidence string
		code                     int
		verdict                  cc.Verdict
	}{
		{"pass", "contract.json", "pass.json", 0, cc.Pass},
		{"fail", "contract.json", "fail.json", 1, cc.Fail},
		{"incomplete", "contract.json", "incomplete.json", 1, cc.Incomplete},
		{"conditional", "waiver-contract.json", "incomplete.json", 3, cc.Conditional},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run([]string{"evaluate", "--contract", "../../examples/" + tt.contract, "--evidence", "../../examples/" + tt.evidence, "--at", "2026-10-04T12:00:00Z", "--format", "json"}, &out, &errOut)
			if code != tt.code {
				t.Fatalf("code %d: %s", code, errOut.String())
			}
			var r cc.Report
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			if r.Verdict != tt.verdict {
				t.Fatalf("got %s", r.Verdict)
			}
			if !strings.Contains(r.Description, "SYNTHETIC NOT GPU VALIDATION") {
				t.Fatal("synthetic label lost")
			}
		})
	}
}

func TestCLIInvalidInputsNeverReturnPass(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"evaluate"}, {"evaluate", "--contract", "missing", "--evidence", "missing", "--at", "2026-10-04T12:00:00Z"}, {"evaluate", "--contract", "../../examples/contract.json", "--evidence", "../../examples/pass.json", "--at", "now"}, {"evaluate", "--contract", "../../examples/contract.json", "--evidence", "../../examples/pass.json", "--at", "2026-10-04T12:00:00Z", "--format", "xml"}, {"evaluate", "--unexpected"}, {"import-nccl", "--input", "missing"}} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 2 {
			t.Fatalf("%v returned %d", args, code)
		}
		if errOut.Len() == 0 {
			t.Fatalf("%v failed without an error", args)
		}
		if out.Len() != 0 {
			t.Fatalf("invalid input emitted a report: %s", out.String())
		}
	}
}

func TestCLIAdapterAndMarkdown(t *testing.T) {
	var out, errOut bytes.Buffer
	args := []string{"import-nccl", "--input", "../../examples/nccl-synthetic.log", "--scope", "communicator/rack-a", "--environment", "synthetic-env-v1", "--recorded-at", "2026-10-04T11:45:00Z", "--reference", "examples/nccl-synthetic.log", "--description", "SYNTHETIC NOT GPU VALIDATION"}
	if code := run(args, &out, &errOut); code != 0 {
		t.Fatalf("code %d: %s", code, errOut.String())
	}
	b, err := cc.DecodeEvidence(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Evidence) != 2 {
		t.Fatal("wrong placement count")
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"evaluate", "--contract", "../../examples/waiver-contract.json", "--evidence", "../../examples/incomplete.json", "--at", "2026-10-04T12:00:00Z"}, &out, &errOut); code != 3 {
		t.Fatalf("code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Verdict: conditional") || !strings.Contains(out.String(), "Waiver: applied") || !strings.Contains(out.String(), "customer-acceptance-owner") {
		t.Fatal("Markdown hides waiver")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic write failure") }
func TestOutputFailureIsNotSuccess(t *testing.T) {
	var errOut bytes.Buffer
	if code := run([]string{"evaluate", "--contract", "../../examples/contract.json", "--evidence", "../../examples/pass.json", "--at", "2026-10-04T12:00:00Z"}, failingWriter{}, &errOut); code != 2 {
		t.Fatalf("output failure returned %d", code)
	}
}

func TestVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"version"}, &out, &errOut); code != 0 || out.String() != buildVersion+"\n" {
		t.Fatal("version failed")
	}
}
