package runner

import (
	"strings"
	"testing"
)

func TestNativeReleaseDiagnosticRedactsBeforeTailBound(t *testing.T) {
	output := strings.Repeat("build output\n", 1000) + "error password=x\nshort=x\nsecret=top-secret-value\npostgres://user:database-password@db/project\nlast useful failure"
	got := nativeReleaseDiagnostic(output, []string{"SSH_PASSWORD=x", "API_TOKEN=top-secret-value", "DATABASE_URL=postgres://user:database-password@db/project"})
	for _, value := range []string{"short=x", "top-secret-value", "database-password"} {
		if strings.Contains(got, value) {
			t.Fatal("credential leaked")
		}
	}
	if len(got) > 6100 || !strings.Contains(got, "earlier output omitted") || !strings.Contains(got, "last useful failure") {
		t.Fatal("lost bounded failure tail")
	}
}
