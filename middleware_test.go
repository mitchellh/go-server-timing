package servertiming

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware(t *testing.T) {
	cases := []struct {
		Name     string
		Metrics  []*Metric
		Expected bool
	}{
		{
			Name:     "nil metrics",
			Metrics:  nil,
			Expected: false,
		},

		{
			Name:     "empty metrics",
			Metrics:  []*Metric{},
			Expected: false,
		},

		{
			Name: "single metric",
			Metrics: []*Metric{
				&Metric{
					Name:     "sql-1",
					Duration: 100 * time.Millisecond,
					Desc:     "MySQL; lookup Server",
				},
			},
			Expected: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set the metrics to the configured case
				h := FromContext(r.Context())
				if h == nil {
					t.Fatal("expected *Header to be present in context")
				}
				h.Metrics = tt.Metrics

				// Write the header to flush the response
				w.WriteHeader(204)
			})

			// Perform the request
			Middleware(handler).ServeHTTP(rec, r)

			// Test that it is present or not
			_, present := map[string][]string(rec.Header())[HeaderKey]
			if present != tt.Expected {
				t.Fatalf("expected header to be present: %v, but wasn't", tt.Expected)
			}

			// Test the response
			expected := (&Header{Metrics: tt.Metrics}).String()
			actual := rec.Header().Get(HeaderKey)
			if actual != expected {
				t.Fatalf("got wrong value, expected != actual: %q != %q", expected, actual)
			}
		})
	}
}
