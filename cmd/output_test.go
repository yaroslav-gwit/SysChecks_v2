package cmd

import "testing"

func TestResolveOutputPrefersExplicitFlag(t *testing.T) {
	defer func() { outputFlag = "" }()

	outputFlag = "json-pretty"
	// --output wins even over a deprecated flag that says something different.
	if got := resolveOutput(outputText, true, false); got != outputJSONPretty {
		t.Fatalf("got %q, want %q", got, outputJSONPretty)
	}
}

// Scripts pinned to the pre-restructure booleans must keep working untouched.
func TestResolveOutputHonoursDeprecatedFlags(t *testing.T) {
	defer func() { outputFlag = "" }()
	outputFlag = ""

	if got := resolveOutput(outputText, true, false); got != outputJSON {
		t.Fatalf("--json: got %q, want %q", got, outputJSON)
	}
	if got := resolveOutput(outputText, false, true); got != outputJSONPretty {
		t.Fatalf("--json-pretty: got %q, want %q", got, outputJSONPretty)
	}
}

// With no flags at all, each command keeps the shape it has always emitted.
func TestResolveOutputFallsBackToCommandDefault(t *testing.T) {
	defer func() { outputFlag = "" }()
	outputFlag = ""

	if got := resolveOutput(outputText, false, false); got != outputText {
		t.Fatalf("got %q, want %q", got, outputText)
	}
	if got := resolveOutput(outputJSON, false, false); got != outputJSON {
		t.Fatalf("got %q, want %q", got, outputJSON)
	}
}

func TestValidateOutputFlag(t *testing.T) {
	defer func() { outputFlag = "" }()

	for _, valid := range append(validOutputFormats(), "") {
		outputFlag = valid
		if err := validateOutputFlag(); err != nil {
			t.Fatalf("%q rejected: %v", valid, err)
		}
	}

	for _, invalid := range []string{"yaml", "JSON", "pretty", "json_pretty"} {
		outputFlag = invalid
		if err := validateOutputFlag(); err == nil {
			t.Fatalf("%q accepted", invalid)
		}
	}
}

// Every advertised completion value must actually be accepted, or tab-completion would
// suggest something the command then rejects.
func TestOutputCompletionsAreAllValid(t *testing.T) {
	defer func() { outputFlag = "" }()

	if len(outputFormatCompletions) != len(validOutputFormats()) {
		t.Fatalf("%d completions for %d valid values", len(outputFormatCompletions), len(validOutputFormats()))
	}
	for _, completion := range outputFormatCompletions {
		value := completion
		for i, r := range completion {
			if r == '\t' {
				value = completion[:i]
				break
			}
		}
		outputFlag = value
		if err := validateOutputFlag(); err != nil {
			t.Fatalf("completion %q is not a valid value: %v", value, err)
		}
	}
}

func TestResolveUpdateScope(t *testing.T) {
	reset := func() {
		applyUpdatesScope = ""
		applyUpdatesCmdSystemUpdates = false
	}
	defer reset()

	tests := []struct {
		name       string
		scope      string
		legacy     bool
		want       string
		wantErrors bool
	}{
		{name: "default is security", want: updateScopeSecurity},
		{name: "explicit security", scope: updateScopeSecurity, want: updateScopeSecurity},
		{name: "explicit system", scope: updateScopeSystem, want: updateScopeSystem},
		{name: "deprecated --system", legacy: true, want: updateScopeSystem},
		{name: "deprecated --system agrees with --scope", scope: updateScopeSystem, legacy: true, want: updateScopeSystem},
		{name: "deprecated --system contradicts --scope", scope: updateScopeSecurity, legacy: true, wantErrors: true},
		{name: "unknown scope", scope: "everything", wantErrors: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset()
			applyUpdatesScope = tt.scope
			applyUpdatesCmdSystemUpdates = tt.legacy

			got, err := resolveUpdateScope()
			if tt.wantErrors {
				if err == nil {
					t.Fatalf("expected an error, got scope %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The scheduled updates job must never guess a policy for the host.
func TestResolveScheduleScopeRequiresExplicitChoice(t *testing.T) {
	defer func() { scheduleScope = "" }()

	scheduleScope = ""
	if _, err := resolveScheduleScope(); err == nil {
		t.Fatal("an unset --scope was accepted; the host's update policy would be guessed")
	}

	for _, valid := range []string{updateScopeSecurity, updateScopeSystem} {
		scheduleScope = valid
		got, err := resolveScheduleScope()
		if err != nil || got != valid {
			t.Fatalf("scope %q: got %q, err %v", valid, got, err)
		}
	}

	scheduleScope = "both"
	if _, err := resolveScheduleScope(); err == nil {
		t.Fatal("invalid scope accepted")
	}
}

// Advertised job completions must be jobs that enable/disable actually know about.
func TestScheduleJobCompletionsResolve(t *testing.T) {
	for _, completion := range scheduleJobCompletions(false) {
		name := completion
		for i, r := range completion {
			if r == '\t' {
				name = completion[:i]
				break
			}
		}
		if _, ok := lookupScheduleJob(name); !ok {
			t.Fatalf("completion %q does not resolve to a job", name)
		}
	}

	if got := scheduleJobCompletions(true); len(got) != len(scheduleJobDefinitions)+1 {
		t.Fatalf("disable completions should include 'all': %#v", got)
	}
}
