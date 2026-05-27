package media

import (
	"regexp"
	"testing"
)

func TestNewUUIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

	for i := 0; i < 10; i++ {
		id := newUUID()
		if !re.MatchString(id) {
			t.Fatalf("invalid uuid format: %s", id)
		}
	}
}
