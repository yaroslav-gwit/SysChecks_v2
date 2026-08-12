package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnsupportedSecurityUpdateDataIsExplicitAndNeverZero(t *testing.T) {
	result := systemUpdatesJsonStruct{
		SystemUpdatesList:   []string{},
		SystemUpdatesStatus: "ok",
		RepositoryIssues:    []repoIssue{},
		SecurityUpdatesList: []string{},
	}
	applySecurityUpdateSupport(&result, false, 0, false, nil)

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(encoded)
	for _, want := range []string{
		`"security_updates":null`,
		`"security_updates_available":null`,
		`"security_updates_list":null`,
		`"security_updates_status":"unsupported"`,
	} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("unsupported state missing %s: %s", want, jsonText)
		}
	}
	if strings.Contains(jsonText, `"security_updates":0`) {
		t.Fatalf("unsupported security state was represented as zero: %s", jsonText)
	}
}

func TestSupportedSecurityUpdateDataRetainsNumericSchema(t *testing.T) {
	result := systemUpdatesJsonStruct{RepositoryIssues: []repoIssue{}}
	applySecurityUpdateSupport(&result, true, 2, true, []string{"musl", "busybox"})

	if count, supported := securityUpdateCount(result); !supported || count != 2 {
		t.Fatalf("securityUpdateCount() = %d, %v", count, supported)
	}
	if result.SecurityUpdatesStatus != "ok" || result.SecurityUpdatesAvailable == nil || !*result.SecurityUpdatesAvailable {
		t.Fatalf("unexpected supported state: %#v", result)
	}
}

func TestAlpineSecurityOnlyApplyIsUnsupported(t *testing.T) {
	if securityOnlyUpdatesSupported(detectOsStruct{manager: packageManagerAPK}) {
		t.Fatal("apk was reported as supporting security-only updates")
	}
	if !securityOnlyUpdatesSupported(detectOsStruct{manager: packageManagerAPT}) {
		t.Fatal("apt security-only support regressed")
	}
}
