package servertiming

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/gddo/httputil/header"
)

// Header is a parsed Server-Timing header value. This can be re-encoded
// and sent as a valid HTTP header value using String().
type Header struct {
	// Metrics is the list of metrics in the header.
	Metrics []*Metric
}

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

// ParseHeader parses a Server-Timing header value.
func ParseHeader(input string) (*Header, error) {
	// Split the comma-separated list of metrics
	rawMetrics := header.ParseList(headerParams(input))

	// Parse the list of metrics. We can pre-allocate the length of the
	// comma-separated list of metrics since at most it will be that and
	// most likely it will be that length.
	metrics := make([]*Metric, 0, len(rawMetrics))
	for _, raw := range rawMetrics {
		var m Metric
		m.Name, m.Extra = header.ParseValueAndParams(headerParams(raw))

		// Description
		if v, ok := m.Extra[paramNameDesc]; ok {
			m.Desc = v
			delete(m.Extra, paramNameDesc)
		}

		// Duration. This is treated as a millisecond value since that
		// is what modern browsers are treating it as. If the parsing of
		// an integer fails, the set value remains in the Extra field.
		if v, ok := m.Extra[paramNameDur]; ok {
			intv, err := strconv.Atoi(v)
			if err == nil {
				m.Duration = time.Duration(intv) * time.Millisecond
				delete(m.Extra, paramNameDur)
			}
		}

		metrics = append(metrics, &m)
	}

	return &Header{Metrics: metrics}, nil
}

// String returns the valid Server-Timing header value that can be
// sent in an HTTP response.
func (h *Header) String() string {
	return ""
}

// fmt.GoStringer so %v works on pointer value.
func (m *Metric) GoString() string {
	if m == nil {
		return "nil"
	}

	return fmt.Sprintf("*%#v", *m)
}

// Specified server-timing-param-name values.
const (
	paramNameDesc = "desc"
	paramNameDur  = "dur"
)

// headerParams is a helper function that takes a header value and turns
// it into the expected argument format for the httputil/header library
// functions..
func headerParams(s string) (http.Header, string) {
	const key = "Key"
	return http.Header(map[string][]string{
		key: []string{s},
	}), key
}
