package proxy

import "testing"

func TestHostsManagerRemoveBlock(t *testing.T) {
	h := NewHostsManager("myproject")

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "removes block",
			content: `127.0.0.1 localhost
# lokl:myproject - START
127.0.0.1 app.example.com
127.0.0.1 api.example.com
# lokl:myproject - END
127.0.0.1 other.host
`,
			want: `127.0.0.1 localhost
127.0.0.1 other.host
`,
		},
		{
			name: "no block present",
			content: `127.0.0.1 localhost
127.0.0.1 other.host
`,
			want: `127.0.0.1 localhost
127.0.0.1 other.host
`,
		},
		{
			name: "block at end",
			content: `127.0.0.1 localhost
# lokl:myproject - START
127.0.0.1 app.example.com
# lokl:myproject - END
`,
			want: `127.0.0.1 localhost
`,
		},
		{
			name: "block at start",
			content: `# lokl:myproject - START
127.0.0.1 app.example.com
# lokl:myproject - END
127.0.0.1 localhost
`,
			want: `127.0.0.1 localhost
`,
		},
		{
			name: "different project unchanged",
			content: `# lokl:otherproject - START
127.0.0.1 other.example.com
# lokl:otherproject - END
`,
			want: `# lokl:otherproject - START
127.0.0.1 other.example.com
# lokl:otherproject - END
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.removeBlock(tt.content)
			if got != tt.want {
				t.Errorf("removeBlock():\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestHostsManagerMarkers(t *testing.T) {
	h := NewHostsManager("testproject")

	if h.startMarker() != "# lokl:testproject - START" {
		t.Errorf("startMarker() = %q", h.startMarker())
	}
	if h.endMarker() != "# lokl:testproject - END" {
		t.Errorf("endMarker() = %q", h.endMarker())
	}
}
