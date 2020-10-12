package servertiming

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	responseBody   = "response"
	responseStatus = http.StatusCreated
)

func TestMiddleware(t *testing.T) {
	cases := []struct {
		Name             string
		Opts             *MiddlewareOpts
		Metrics          []*Metric
		SkipWriteHeaders bool
		Expected         bool
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
			Name: "single metric disable headers option",
			Opts: &MiddlewareOpts{DisableHeaders: true},
			Metrics: []*Metric{
				{
					Name:     "sql-1",
					Duration: 100 * time.Millisecond,
					Desc:     "MySQL; lookup Server",
				},
			},
			Expected: false,
		},

		{
			Name: "single metric",
			Metrics: []*Metric{
				{
					Name:     "sql-1",
					Duration: 100 * time.Millisecond,
					Desc:     "MySQL; lookup Server",
				},
			},
			Expected: true,
		},

		{
			Name: "single metric without invoking WriteHeaders in handler",
			Metrics: []*Metric{
				{
					Name:     "sql-1",
					Duration: 100 * time.Millisecond,
					Desc:     "MySQL; lookup Server",
				},
			},
			Expected:         true,
			SkipWriteHeaders: true,
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
				if !tt.SkipWriteHeaders {
					w.WriteHeader(responseStatus)
				}

				// Write date to response body
				w.Write([]byte(responseBody))
			})

			// Perform the request
			Middleware(handler, tt.Opts).ServeHTTP(rec, r)

			// Test that it is present or not
			_, present := map[string][]string(rec.Header())[HeaderKey]
			if present != tt.Expected {
				t.Fatalf("expected header to be present: %v, but wasn't", tt.Expected)
			}

			// Test the response headers
			expected := (&Header{Metrics: tt.Metrics}).String()
			if tt.Opts != nil && tt.Opts.DisableHeaders == true {
				expected = ""
			}
			actual := rec.Header().Get(HeaderKey)
			if actual != expected {
				t.Fatalf("got wrong value, expected != actual: %q != %q", expected, actual)
			}

			// Test the status code of the response, if we skip the write headers method, the default 200 should be
			// the response status code
			expectedStatus := responseStatus
			if tt.SkipWriteHeaders {
				expectedStatus = http.StatusOK
			}
			if actualStatus := rec.Result().StatusCode; expectedStatus != actualStatus {
				t.Fatalf("got unexpected status code value, expected != actual: %q != %q", expectedStatus, actualStatus)
			}

			// Test the response body was left intact
			if responseBody != rec.Body.String() {
				t.Fatalf("got unexpected body, expected != actual: %q != %q", responseBody, rec.Body.String())
			}
		})
	}
}

// We need to test this separately since the httptest.ResponseRecorder
// doesn't properly reflect that headers can't be set after writing data,
// so we have to use a real server.
func TestMiddleware_writeHeaderNotCalled(t *testing.T) {
	metrics := []*Metric{
		{
			Name:     "sql-1",
			Duration: 100 * time.Millisecond,
			Desc:     "MySQL; lookup Server",
		},
	}

	// Start our test server
	ts := httptest.NewServer(Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the metrics to the configured case
		h := FromContext(r.Context())
		if h == nil {
			t.Fatal("expected *Header to be present in context")
		}

		h.Metrics = metrics

		// Write date to response body WITHOUT calling WriteHeader
		w.Write([]byte(responseBody))
	}), nil))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Test that it is present or not
	_, present := map[string][]string(res.Header)[HeaderKey]
	if !present {
		t.Fatal("expected header to be present")
	}

	// Test the response headers
	expected := (&Header{Metrics: metrics}).String()
	actual := res.Header.Get(HeaderKey)
	if actual != expected {
		t.Fatalf("got wrong value, expected != actual: %q != %q", expected, actual)
	}
}
