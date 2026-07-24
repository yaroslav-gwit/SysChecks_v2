package cmd

import (
	"fmt"
	"sort"
	"strings"
	"syschecks/helpers"

	"github.com/spf13/cobra"
)

// The `schedule` group replaces the per-job `--disable` flags with real enable/disable
// verbs, and puts the verb before the job name so `schedule enable <TAB>` can complete the
// job list. `cron` remains an alias of the same command, and its old subcommands are kept
// (hidden) so deployed scripts keep working.

// scheduleJob names a unit of automation. These strings are the tab-completable values of
// `schedule enable|disable <job>`.
const (
	scheduleJobUpdates     = "updates"
	scheduleJobSelfUpdate  = "self-update"
	scheduleJobKernelClean = "kernel-cleanup"
	scheduleJobUpdateCache = "update-cache"
	scheduleJobAll         = "all"
	scheduleDefaultKeep    = 4
)

type scheduleJobDefinition struct {
	name        string
	description string
	enable      func() error
	disable     func()
}

var scheduleJobDefinitions = []scheduleJobDefinition{
	{
		name:        scheduleJobUpdateCache,
		description: "Refresh the cached update report (banner + monitoring source)",
		enable: func() error {
			helpers.CacheCreate()
			return nil
		},
		disable: helpers.CacheDisable,
	},
	{
		name:        scheduleJobUpdates,
		description: "Install updates automatically (requires --scope)",
		enable: func() error {
			scope, err := resolveScheduleScope()
			if err != nil {
				return err
			}
			if scope == updateScopeSystem {
				helpers.SystemUpdates()
			} else {
				helpers.SecurityUpdates()
			}
			return nil
		},
		disable: helpers.UpdatesDisable,
	},
	{
		name:        scheduleJobSelfUpdate,
		description: "Keep syschecks itself updated to the latest release",
		enable: func() error {
			helpers.AutoUpdateEnable()
			return nil
		},
		disable: helpers.AutoUpdateDisable,
	},
	{
		name:        scheduleJobKernelClean,
		description: "Remove old kernel packages weekly (--keep N)",
		enable: func() error {
			helpers.KernelCleanupEnable(scheduleKeep)
			return nil
		},
		disable: helpers.KernelCleanupDisable,
	},
}

var (
	scheduleScope string
	scheduleKeep  int

	scheduleCmd = &cobra.Command{
		Use:     "schedule",
		Aliases: []string{"cron"},
		Short:   "Manage scheduled syschecks jobs",
		Long:    `Show which syschecks jobs run automatically, and enable or disable them.`,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printCronStatus()
		},
	}

	scheduleListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"status"},
		Short:   "Show every scheduled job, whether it is enabled, and its schedule",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printCronStatus()
		},
	}

	scheduleEnableCmd = &cobra.Command{
		Use:       "enable <job>",
		Short:     "Enable a scheduled job",
		Long:      "Enable a scheduled job.\n\n" + scheduleJobHelp(),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: scheduleJobCompletions(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			definition, ok := lookupScheduleJob(args[0])
			if !ok {
				return fmt.Errorf("unknown job %q; valid jobs: %s", args[0], strings.Join(scheduleJobNames(false), ", "))
			}
			return definition.enable()
		},
	}

	scheduleDisableCmd = &cobra.Command{
		Use:       "disable <job>",
		Short:     "Disable a scheduled job",
		Long:      "Disable a scheduled job, or 'all' to remove every syschecks job.\n\n" + scheduleJobHelp(),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: scheduleJobCompletions(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == scheduleJobAll {
				for _, definition := range scheduleJobDefinitions {
					definition.disable()
				}
				return nil
			}
			definition, ok := lookupScheduleJob(args[0])
			if !ok {
				return fmt.Errorf("unknown job %q; valid jobs: %s", args[0], strings.Join(scheduleJobNames(true), ", "))
			}
			definition.disable()
			return nil
		},
	}
)

// resolveScheduleScope requires an explicit choice. Defaulting would silently pick an update
// policy for the host, which is exactly the ambiguity the old three-boolean form created.
func resolveScheduleScope() (string, error) {
	switch scheduleScope {
	case updateScopeSecurity, updateScopeSystem:
		return scheduleScope, nil
	case "":
		return "", fmt.Errorf("--scope is required for the %q job: choose %s or %s",
			scheduleJobUpdates, updateScopeSecurity, updateScopeSystem)
	default:
		return "", fmt.Errorf("invalid --scope %q: must be %s or %s", scheduleScope, updateScopeSecurity, updateScopeSystem)
	}
}

func lookupScheduleJob(name string) (scheduleJobDefinition, bool) {
	for _, definition := range scheduleJobDefinitions {
		if definition.name == name {
			return definition, true
		}
	}
	return scheduleJobDefinition{}, false
}

func scheduleJobNames(includeAll bool) []string {
	names := make([]string, 0, len(scheduleJobDefinitions)+1)
	for _, definition := range scheduleJobDefinitions {
		names = append(names, definition.name)
	}
	sort.Strings(names)
	if includeAll {
		names = append(names, scheduleJobAll)
	}
	return names
}

func scheduleJobHelp() string {
	var builder strings.Builder
	builder.WriteString("Jobs:\n")
	for _, definition := range scheduleJobDefinitions {
		builder.WriteString(fmt.Sprintf("  %-15s %s\n", definition.name, definition.description))
	}
	return builder.String()
}

// scheduleJobCompletions supplies descriptions alongside the job names in shells that render
// them (cobra's bash V2, zsh, fish).
func scheduleJobCompletions(includeAll bool) []string {
	completions := make([]string, 0, len(scheduleJobDefinitions)+1)
	for _, definition := range scheduleJobDefinitions {
		completions = append(completions, definition.name+"\t"+definition.description)
	}
	if includeAll {
		completions = append(completions, scheduleJobAll+"\tEvery syschecks job")
	}
	return completions
}
