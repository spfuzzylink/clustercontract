package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	cc "github.com/spfuzzylink/clustercontract"
)

var buildVersion = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: clustercontract evaluate --contract FILE --evidence FILE --at RFC3339 [--format markdown|json]\n       clustercontract import-nccl --input FILE --scope SCOPE --environment FINGERPRINT --recorded-at RFC3339 --reference REF [--description TEXT]")
		return 2
	}
	var code int
	var err error
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return 2
		}
		_, err = fmt.Fprintln(out, buildVersion)
	case "evaluate":
		code, err = evaluate(args[1:], out, errOut)
	case "import-nccl":
		code, err = importNCCL(args[1:], out, errOut)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
		code = 2
	}
	if err != nil {
		fmt.Fprintln(errOut, "clustercontract:", err)
		return 2
	}
	return code
}

func evaluate(args []string, out, errOut io.Writer) (int, error) {
	fs := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	fs.SetOutput(errOut)
	contractPath := fs.String("contract", "", "contract JSON file")
	evidencePath := fs.String("evidence", "", "normalized evidence JSON file")
	atText := fs.String("at", "", "explicit RFC3339 evaluation time")
	format := fs.String("format", "markdown", "markdown or json")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments")
	}
	if *contractPath == "" || *evidencePath == "" || *atText == "" {
		return 2, fmt.Errorf("--contract, --evidence and --at are required")
	}
	if *format != "markdown" && *format != "json" {
		return 2, fmt.Errorf("--format must be markdown or json")
	}
	at, err := time.Parse(time.RFC3339Nano, *atText)
	if err != nil {
		return 2, fmt.Errorf("--at: %w", err)
	}
	f, err := os.Open(*contractPath)
	if err != nil {
		return 2, fmt.Errorf("open contract: %w", err)
	}
	c, err := cc.DecodeContract(f)
	closeErr := f.Close()
	if err != nil {
		return 2, err
	}
	if closeErr != nil {
		return 2, closeErr
	}
	f, err = os.Open(*evidencePath)
	if err != nil {
		return 2, fmt.Errorf("open evidence: %w", err)
	}
	b, err := cc.DecodeEvidence(f)
	closeErr = f.Close()
	if err != nil {
		return 2, err
	}
	if closeErr != nil {
		return 2, closeErr
	}
	report, err := cc.Evaluate(c, b, at)
	if err != nil {
		return 2, err
	}
	if *format == "json" {
		err = writeJSON(out, report)
	} else {
		err = cc.WriteMarkdown(out, report)
	}
	return report.ExitCode(), err
}

func importNCCL(args []string, out, errOut io.Writer) (int, error) {
	fs := flag.NewFlagSet("import-nccl", flag.ContinueOnError)
	fs.SetOutput(errOut)
	path := fs.String("input", "", "all_reduce_perf stdout file")
	scope := fs.String("scope", "", "complete communicator scope")
	environment := fs.String("environment", "", "asserted environment fingerprint")
	recorded := fs.String("recorded-at", "", "asserted RFC3339 observation time")
	reference := fs.String("reference", "", "provenance reference")
	description := fs.String("description", "", "evidence description")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected positional arguments")
	}
	if *path == "" {
		return 2, fmt.Errorf("--input is required")
	}
	at, err := time.Parse(time.RFC3339Nano, *recorded)
	if err != nil {
		return 2, fmt.Errorf("--recorded-at: %w", err)
	}
	f, err := os.Open(*path)
	if err != nil {
		return 2, fmt.Errorf("open NCCL log: %w", err)
	}
	defer f.Close()
	b, err := cc.ImportNCCL(f, cc.NCCLOptions{Scope: *scope, EnvironmentFingerprint: *environment, RecordedAt: at, Reference: *reference, Description: *description})
	if err != nil {
		return 2, err
	}
	return 0, writeJSON(out, b)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
