package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
)

// outputFormat is the single knob that replaced the per-command --json / --json-pretty
// flags. Each command keeps its historical default, so an existing caller that passes no
// flag sees exactly what it saw before.
type outputFormat string

const (
	outputText       outputFormat = "text"
	outputJSON       outputFormat = "json"
	outputJSONPretty outputFormat = "json-pretty"
)

// outputFlag holds the raw --output value. Empty means "use the command's default".
var outputFlag string

// outputFormatCompletions feeds shell completion. The text after \t is rendered as a
// description by cobra's bash V2, zsh and fish completion scripts.
var outputFormatCompletions = []string{
	"text\tHuman-readable output",
	"json\tCompact JSON",
	"json-pretty\tIndented JSON",
}

func validOutputFormats() []string {
	return []string{string(outputText), string(outputJSON), string(outputJSONPretty)}
}

// validateOutputFlag runs before any command executes, so a typo fails immediately instead
// of silently falling back to a default the caller did not ask for.
func validateOutputFlag() error {
	if outputFlag == "" {
		return nil
	}
	for _, candidate := range validOutputFormats() {
		if outputFlag == candidate {
			return nil
		}
	}
	return fmt.Errorf("invalid --output %q: must be one of %s", outputFlag, strings.Join(validOutputFormats(), ", "))
}

// resolveOutput picks the effective format. The deprecated per-command booleans still win
// when --output is absent, so scripts pinned to --json-pretty keep working unchanged.
func resolveOutput(def outputFormat, legacyJSON bool, legacyPretty bool) outputFormat {
	if outputFlag != "" {
		return outputFormat(outputFlag)
	}
	if legacyPretty {
		return outputJSONPretty
	}
	if legacyJSON {
		return outputJSON
	}
	return def
}

// emitJSON prints a value in the requested JSON shape. outputText is treated as compact
// JSON for commands whose only representation is JSON (kernel status, updates check).
func emitJSON(value any, format outputFormat) {
	var encoded []byte
	var err error

	if format == outputJSONPretty {
		encoded, err = json.MarshalIndent(value, "", "   ")
	} else {
		encoded, err = json.Marshal(value)
	}
	if err != nil {
		log.Fatalf("Error marshaling output: %v", err)
	}
	fmt.Println(string(encoded))
}

func registerOutputCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("output", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return outputFormatCompletions, cobra.ShellCompDirectiveNoFileComp
	})
}
