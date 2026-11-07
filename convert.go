package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// --- Firestore typed value <-> Go value conversion ---

// typedToGo converts a Firestore typed value (string/integerValue/mapValue/...)
// into plain Go values (string, int64, float64, bool, map[string]any, []any).
func typedToGo(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if f, ok := t["fields"].(map[string]any); ok {
			return typedToGoFields(f)
		}
		return typedToGoFields(t) // already-field form
	default:
		return v
	}
}

func typedToGoFields(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = typedToGoValue(v)
	}
	return out
}

func typedToGoValue(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	switch {
	case m["stringValue"] != nil:
		return m["stringValue"]
	case m["integerValue"] != nil:
		switch n := m["integerValue"].(type) {
		case string:
			if i, err := strconv.ParseInt(n, 10, 64); err == nil {
				return i
			}
		case float64:
			return int64(n)
		}
		return m["integerValue"]
	case m["doubleValue"] != nil:
		if f, ok := m["doubleValue"].(float64); ok {
			return f
		}
		return m["doubleValue"]
	case m["booleanValue"] != nil:
		return m["booleanValue"]
	case m["timestampValue"] != nil:
		return m["timestampValue"]
	case m["nullValue"] != nil:
		return nil
	case m["arrayValue"] != nil:
		arr, _ := m["arrayValue"].(map[string]any)
		vals, _ := arr["values"].([]any)
		out := make([]any, len(vals))
		for i, e := range vals {
			out[i] = typedToGoValue(e)
		}
		return out
	case m["mapValue"] != nil:
		mm, _ := m["mapValue"].(map[string]any)
		f, _ := mm["fields"].(map[string]any)
		return typedToGoFields(f)
	default:
		return v // unknown or empty map (e.g. nullValue: 0) -> keep as-is
	}
}

// goToTyped converts a plain Go value tree back into Firestore typed values.
func goToTyped(v any) (map[string]any, error) {
	top, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top-level value is not a map")
	}
	fields := make(map[string]any, len(top))
	for k, val := range top {
		tv, err := goValueToTyped(val)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", k, err)
		}
		fields[k] = tv
	}
	return fields, nil
}

func goValueToTyped(v any) (any, error) {
	switch t := v.(type) {
	case nil:
		return map[string]any{"nullValue": nil}, nil
	case string:
		return map[string]any{"stringValue": t}, nil
	case int:
		return map[string]any{"integerValue": strconv.Itoa(t)}, nil
	case int64:
		return map[string]any{"integerValue": strconv.FormatInt(t, 10)}, nil
	case int32:
		return map[string]any{"integerValue": strconv.FormatInt(int64(t), 10)}, nil
	case float64:
		if t == float64(int64(t)) && t < 1e15 && t > -1e15 {
			// emit as double to preserve type
			b, _ := json.Marshal(t)
			return map[string]any{"doubleValue": json.Number(string(b))}, nil
		}
		b, _ := json.Marshal(t)
		return map[string]any{"doubleValue": json.Number(string(b))}, nil
	case bool:
		return map[string]any{"booleanValue": t}, nil
	case map[string]any:
		fields := make(map[string]any, len(t))
		for k, val := range t {
			tv, err := goValueToTyped(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			fields[k] = tv
		}
		return map[string]any{"mapValue": map[string]any{"fields": fields}}, nil
	case []any:
		vals := make([]any, len(t))
		for i, e := range t {
			tv, err := goValueToTyped(e)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			vals[i] = tv
		}
		return map[string]any{"arrayValue": map[string]any{"values": vals}}, nil
	default:
		return map[string]any{"stringValue": fmt.Sprint(t)}, nil
	}
}

// leafToEditable renders a leaf value as text for the edit input.
func leafToEditable(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []any:
		if len(t) == 0 {
			return ""
		}
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = fmt.Sprint(e)
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(t)
	}
}

// parseEditInput converts the user's edit text back into a Go value,
// inferring the type from the existing value.
func parseEditInput(existing any, input string) any {
	switch existing.(type) {
	case []any:
		var out []any
		for _, p := range strings.Split(input, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	case int64, int, int32:
		if i, err := strconv.ParseInt(input, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(input, 64); err == nil {
			return f
		}
		return input
	case float64:
		if f, err := strconv.ParseFloat(input, 64); err == nil {
			return f
		}
		return input
	case bool:
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0", "":
			return false
		}
		return input
	default:
		return input
	}
}

func sliceContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
