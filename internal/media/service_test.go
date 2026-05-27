package media

import "testing"

func TestNormalizeAndValidateMIME(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		declared string
		sniff    []byte
		wantMIME string
		wantKind string
		wantErr  bool
	}{
		{
			name:     "jpeg by sniff",
			fileName: "a.bin",
			sniff:    []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
			wantMIME: "image/jpeg",
			wantKind: "image",
		},
		{
			name:     "mov by extension fallback",
			fileName: "video.mov",
			sniff:    []byte("unknown"),
			wantMIME: "video/quicktime",
			wantKind: "video",
		},
		{
			name:     "invalid",
			fileName: "doc.pdf",
			sniff:    []byte("%PDF"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMIME, gotKind, err := normalizeAndValidateMIME(tt.fileName, tt.declared, tt.sniff)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMIME != tt.wantMIME {
				t.Fatalf("want mime=%q got=%q", tt.wantMIME, gotMIME)
			}
			if gotKind != tt.wantKind {
				t.Fatalf("want kind=%q got=%q", tt.wantKind, gotKind)
			}
		})
	}
}
