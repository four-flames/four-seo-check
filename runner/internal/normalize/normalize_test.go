package normalize

import (
	"net/url"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "already clean URL",
			input: "https://example.com/page",
			want:  "https://example.com/page",
		},
		{
			name:  "trailing slash removal",
			input: "https://example.com/page/",
			want:  "https://example.com/page",
		},
		{
			name:  "root path",
			input: "https://example.com/",
			want:  "https://example.com/",
		},
		{
			name:  "fragment removal",
			input: "https://example.com/page#section",
			want:  "https://example.com/page",
		},
		{
			name:  "default port removal https",
			input: "https://example.com:443/page",
			want:  "https://example.com/page",
		},
		{
			name:  "default port removal http",
			input: "http://example.com:80/page",
			want:  "http://example.com/page",
		},
		{
			name:  "www removal",
			input: "https://www.example.com/page",
			want:  "https://example.com/page",
		},
		{
			name:  "query param sorting",
			input: "https://example.com/page?b=2&a=1",
			want:  "https://example.com/page?a=1&b=2",
		},
		{
			name:  "uppercase scheme and host",
			input: "HTTPS://Example.COM/Page",
			want:  "https://example.com/Page",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "non-http scheme",
			input: "ftp://example.com/file",
			want:  "",
		},
		{
			name:  "javascript URL",
			input: "javascript:void(0)",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSameHost(t *testing.T) {
	tests := []struct {
		name  string
		url1  string
		url2  string
		want  bool
	}{
		{
			name: "same host with www difference",
			url1: "https://www.example.com",
			url2: "https://example.com",
			want: true,
		},
		{
			name: "different hosts",
			url1: "https://example.com",
			url2: "https://other.com",
			want: false,
		},
		{
			name: "first URL nil",
			url1: "",
			url2: "https://example.com",
			want: false,
		},
		{
			name: "second URL nil",
			url1: "https://example.com",
			url2: "",
			want: false,
		},
		{
			name: "both URLs nil",
			url1: "",
			url2: "",
			want: false,
		},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u1, u2 *url.URL
			if tt.url1 != "" {
				var err error
				u1, err = url.Parse(tt.url1)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tt.url2 != "" {
				var err error
				u2, err = url.Parse(tt.url2)
				if err != nil {
					t.Fatal(err)
				}
			}
			got := SameHost(u1, u2)
			if got != tt.want {
				t.Errorf("SameHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
