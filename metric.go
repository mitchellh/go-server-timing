package servertiming

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Metric represents a single metric for the Server-Timing header.
type Metric struct {
	// Name is the name of the metric. This must be a valid RFC7230 "token"
	// format. In a gist, this is an alphanumeric string that may contain
	// most common symbols but may not contain any whitespace. The exact
	// syntax can be found in RFC7230.
	//
	// It is common for Name to be a unique identifier (such as "sql-1") and
	// for a more human-friendly name to be used in the "desc" field.
	Name string

	// Duration is the duration of this Metric.
	Duration time.Duration

	// Desc is any string describing this metric. For example: "SQL Primary".
	// The specific format of this is `token | quoted-string` according to
	// RFC7230.
	Desc string

	// Extra is a set of extra parameters and values to send with the
	// metric. The specification states that unrecognized parameters are
	// to be ignored so it should be safe to add additional data here. The
	// key must be a valid "token" (same syntax as Name) and the value can
	// be any "token | quoted-string" (same as Desc field).
	//
	// If this map contains a key that would be sent by another field in this
	// struct (such as "desc"), then this value is prioritized over the
	// struct value.
	Extra map[string]string
}

// String returns the valid Server-Timing metric entry value.
func (m *Metric) String() string {
	// Begin building parts, expected capacity is length of extra
	// fields plus id, desc, dur.
	parts := make([]string, 1, len(m.Extra)+3)
	parts[0] = m.Name

	// Description
	if _, ok := m.Extra[paramNameDesc]; !ok && m.Desc != "" {
		parts = append(parts, headerEncodeParam(paramNameDesc, m.Desc))
	}

	// Duration
	if _, ok := m.Extra[paramNameDur]; !ok && m.Duration > 0 {
		parts = append(parts, headerEncodeParam(
			paramNameDur, strconv.Itoa(int(m.Duration/time.Millisecond))))
	}

	// All remaining extra params
	for k, v := range m.Extra {
		parts = append(parts, headerEncodeParam(k, v))
	}

	return strings.Join(parts, ";")
}

// fmt.GoStringer so %v works on pointer value.
func (m *Metric) GoString() string {
	if m == nil {
		return "nil"
	}

	return fmt.Sprintf("*%#v", *m)
}
