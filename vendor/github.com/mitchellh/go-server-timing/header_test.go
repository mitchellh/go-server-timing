package servertiming

import (
	"reflect"
	"testing"
	"time"
)

// headerCases contains test cases for the Server-Timing header. This set
// of test cases is used to test both parsing the header value as well as
// generating the correct header value.
var headerCases = []struct {
	Metrics     []*Metric
	HeaderValue string
}{
	{
		[]*Metric{
			&Metric{
				Name:     "sql-1",
				Duration: 100 * time.Millisecond,
				Desc:     "MySQL lookup Server",
				Extra:    map[string]string{},
			},
		},
		`sql-1;desc="MySQL lookup Server";dur=100`,
	},

	// Comma in description
	{
		[]*Metric{
			&Metric{
				Name:     "sql-1",
				Duration: 100 * time.Millisecond,
				Desc:     "MySQL, lookup Server",
				Extra:    map[string]string{},
			},
		},
		`sql-1;desc="MySQL, lookup Server";dur=100`,
	},

	// Semicolon in description
	{
		[]*Metric{
			&Metric{
				Name:     "sql-1",
				Duration: 100 * time.Millisecond,
				Desc:     "MySQL; lookup Server",
				Extra:    map[string]string{},
			},
		},
		`sql-1;desc="MySQL; lookup Server";dur=100`,
	},
}

func TestParseHeader(t *testing.T) {
	for _, tt := range headerCases {
		t.Run(tt.HeaderValue, func(t *testing.T) {
			h, err := ParseHeader(tt.HeaderValue)
			if err != nil {
				t.Fatalf("error parsing header: %s", err)
			}

			if !reflect.DeepEqual(h.Metrics, tt.Metrics) {
				t.Fatalf("received, expected:\n\n%#v\n\n%#v", h.Metrics, tt.Metrics)
			}
		})
	}
}

func TestHeaderString(t *testing.T) {
	for _, tt := range headerCases {
		t.Run(tt.HeaderValue, func(t *testing.T) {
			h := &Header{Metrics: tt.Metrics}
			actual := h.String()
			if actual != tt.HeaderValue {
				t.Fatalf("received, expected:\n\n%q\n\n%q", actual, tt.HeaderValue)
			}
		})
	}
}

// Same as TestHeaderString but using the Add method
func TestHeaderAdd(t *testing.T) {
	for _, tt := range headerCases {
		t.Run(tt.HeaderValue, func(t *testing.T) {
			var h Header
			for _, m := range tt.Metrics {
				h.Add(m)
			}

			actual := h.String()
			if actual != tt.HeaderValue {
				t.Fatalf("received, expected:\n\n%q\n\n%q", actual, tt.HeaderValue)
			}
		})
	}
}
