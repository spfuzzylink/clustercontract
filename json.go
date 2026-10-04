package clustercontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

const MaxInputBytes = 16 << 20

func DecodeContract(r io.Reader) (Contract, error) {
	var c Contract
	if err := decodeStrict(r, &c); err != nil {
		return c, fmt.Errorf("contract JSON: %w", err)
	}
	return c, c.Validate()
}

func DecodeEvidence(r io.Reader) (EvidenceBundle, error) {
	var b EvidenceBundle
	if err := decodeStrict(r, &b); err != nil {
		return b, fmt.Errorf("evidence JSON: %w", err)
	}
	return b, b.Validate()
}

func decodeStrict(r io.Reader, out any) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxInputBytes {
		return fmt.Errorf("input exceeds %d bytes", MaxInputBytes)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := readValue(d, 0)
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing input: %w", err)
	}
	if err := checkFields(v, reflect.TypeOf(out).Elem(), "$"); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

func readValue(d *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("JSON nesting exceeds 64 levels")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("null is not allowed; omit optional fields")
	}
	switch t {
	case json.Delim('{'):
		m := make(map[string]any)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			k, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("object key must be a string")
			}
			if _, ok := m[k]; ok {
				return nil, fmt.Errorf("duplicate JSON key %q", k)
			}
			v, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		if _, err := d.Token(); err != nil {
			return nil, err
		}
		return m, nil
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		if _, err := d.Token(); err != nil {
			return nil, err
		}
		return a, nil
	default:
		return t, nil
	}
}

// encoding/json accepts case-insensitive field names; the wire format does not.
func checkFields(v any, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		fields := make(map[string]reflect.Type)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fields[strings.Split(f.Tag.Get("json"), ",")[0]] = f.Type
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %s.%s", path, k)
			}
			if err := checkFields(m[k], ft, path+"."+k); err != nil {
				return err
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		for i, e := range a {
			if err := checkFields(e, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
