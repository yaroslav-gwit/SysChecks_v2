# CLI restructure proposal

Status: **IMPLEMENTED** (revision 3). Shipped in the Unreleased section of `CHANGELOG.md`.

This document is kept as the design record: it explains *why* the surface looks the way it
does. For how to use the result, see `AgentDocs/COMMANDS.md`. Every decision below was
implemented as written, with one addition noted under "Breaking change".

## Why

The surface grew command-by-command, so the same idea is now expressed three different ways
depending on where you are. The concrete collisions:

### 1. "updates" is a noun with three different verbs attached, in three different grammars

| Command | Actually does | Grammar |
|---|---|---|
| `syschecks updates` | reports what is available | noun only |
| `syschecks apply-updates` | installs them | verb-noun |
| `syschecks cron updates` | schedules the install | group-noun |

Three spellings of one resource. Nothing tells you which one mutates the system.

### 2. `--disable` is a subcommand wearing a flag costume

`cron init --disable` *removes* what `init` creates. Same for `cron updates --disable`,
`cron autoupdate --disable`, `cron kernels --disable`. A flag whose job is to invert the
command's verb is a second verb.

### 3. Three booleans that are secretly one enum

`cron updates` accepts `--security`, `--system`, `--disable`, then rejects the combination at
runtime:

```go
if selected > 1 {
    return fmt.Errorf("choose exactly one of --security, --system, or --disable")
}
```

If exactly one is required, it is one value, not three flags.

### 4. `--system` is not the same kind of flag in the two places it appears

- `apply-updates --system` — optional; without it you get security-only.
- `cron updates --system` — effectively required; omitting every flag prints help and does nothing.

Same name, same apparent meaning, different requiredness and different failure mode.

### 5. Mutation safety is opt-in in one place and unavailable everywhere else

- `kernel cleanup` is dry-run by default and needs `--execute` to act.
- `apply-updates` installs immediately and has **no** dry-run at all — the more destructive
  of the two is the less guarded one.
- `release.sh` uses a third convention, `-n/--dry-run`.

### 6. JSON flags differ per command

| Command | Flags | Default output |
|---|---|---|
| `updates`, `kernel` | `--json-pretty` | JSON always |
| `users` | `--json` *and* `--json-pretty` | table |
| `sysinfo` | none | plain text |
| `banner`, `cron status` | none | human only |

Two booleans on `users` encode one three-valued choice, and `--json-pretty` means "pretty"
on one command and "JSON at all" on another.

### 7. The cron subcommand rarely matches the command it schedules

| Scheduler | Schedules |
|---|---|
| `cron updates` | `apply-updates` |
| `cron autoupdate` | `self-update` |
| `cron kernels` | `kernel cleanup` |
| `cron init` | `updates --cache-create` |

`cron init` is the worst of these: the name says nothing about what it initialises.

### 8. `--cache-use` defaults to true

`syschecks updates` returns cached data by default, so it can report a stale answer while
looking like a live check. `--cache-create` additionally turns the same command from
"print a report" into "write a file and print nothing".

---

## Proposed structure

Design rules:

1. **Resource, then verb.** `syschecks <resource> <verb>`.
2. **An action with an inverse gets `enable`/`disable`, never a `--disable` flag.**
3. **One scheduling view.** Keep a single place that answers "what runs automatically on this
   box?" — today's `cron status`.
4. **One output flag, globally, with shell completion on its values.**
5. **`--dry-run` everywhere something is mutated**, never `--execute`.
6. **Never move a command that something outside this repo invokes by path.** `banner` is
   called from `/etc/profile.d` on every deployed host; moving it would mean editing a
   profile file on every Linux system we manage. It stays at the top level.

```
syschecks
├── banner                    --no-emojis  --all  --disk-used-threshold
│                             -o json      ← replaces `sysinfo` entirely
├── updates
│   ├── check                 --refresh | --cached           (was: updates)
│   ├── apply                 --scope security|system        (was: apply-updates)
│   │                         --delay  --no-delay  --dry-run  --ignore-locks
│   └── refresh                                              (was: updates --cache-create)
│       └── alias: `updates cache refresh`
├── kernel
│   ├── status                                               (was: kernel)
│   └── cleanup               --keep  --dry-run              (was: kernel cleanup --execute)
├── users
│   └── list                  --all                          (was: users / userinfo)
├── schedule                                                 (was: cron)
│   ├── list                                                 (was: cron status)
│   ├── enable  <job>         --scope security|system  --keep
│   └── disable <job>
├── migrate                   --apply                        ← new, see Migration
├── self-update               --check  --force
├── zabbix init
└── version                   -v
```

### `sysinfo` is deleted, not moved

There is no `system` command group. Everything `sysinfo` reported becomes part of
`banner -o json`, which means:

- No profile file on any host has to change.
- An operator who wants the machine-readable view of *anything* on the banner — IPs, CPU,
  RAM, uptime, disks, kernel state, update counts, repository health — gets it from one
  command instead of guessing which subcommand owns which field.
- The human banner and the JSON view cannot drift apart, because they are one code path.

`banner -o json` should emit the full structure regardless of the exception-only display
rules, i.e. it behaves like `--all` implicitly. A healthy check that is hidden from the
human banner still needs to be present as `"healthy": true` for Zabbix to alert on its
absence.

### Global output flag

```
-o, --output text|json|json-pretty
```

Persistent on the root command. Per-command defaults stay as they are today
(`updates check` and `kernel status` default to `json`; `users list` and `banner` default to
`text`), so no existing consumer changes shape unless it asks.

**Value completion is a requirement, not a nice-to-have.** Cobra supports this directly:

```go
rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format")
rootCmd.RegisterFlagCompletionFunc("output",
    func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
        return []string{
            "text\tHuman-readable output",
            "json\tCompact JSON",
            "json-pretty\tIndented JSON",
        }, cobra.ShellCompDirectiveNoFileComp
    })
```

The `\t`-suffixed descriptions render in the completion menu. Cobra emits its V2 bash script,
which formats and aligns descriptions, so this works in bash as well as zsh and fish.

Job names must complete the same way, which is why `schedule enable <job>` puts the verb
first — `schedule enable <TAB>` offers the job list:

```go
scheduleEnableCmd.ValidArgs = []string{
    "updates", "self-update", "kernel-cleanup", "update-cache",
}
scheduleEnableCmd.Args = cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)
```

**Installation is already handled.** `auto-install.sh` and `install-offline.sh` both call
`enable_bash_completion`, which writes `syschecks completion bash` to
`/etc/bash_completion.d/syschecks`. Two properties of that existing code matter here:

- It runs on **every** install *and* update — `enable_bash_completion` is invoked before the
  `IS_UPDATE` branch — so the completion script is regenerated from the new binary on upgrade.
  New subcommands, job names, and `--output` values therefore become completable without any
  extra migration step.
- Only bash is installed today. `syschecks completion zsh|fish|powershell` still generate on
  demand, so operators on those shells can wire them up manually; adding them to the installer
  is optional and independent of this proposal.

### Mapping

| Today | Proposed |
|---|---|
| `updates` | `updates check --cached` |
| `updates --cache-create` | `updates refresh` (alias: `updates cache refresh`) |
| `updates --json-pretty` | `updates check -o json-pretty` |
| `apply-updates` | `updates apply --scope security` |
| `apply-updates --system` | `updates apply --scope system` |
| `apply-updates -i` | `updates apply --ignore-locks` |
| `kernel` | `kernel status` |
| `kernel cleanup --execute` | `kernel cleanup` |
| `kernel cleanup` (preview) | `kernel cleanup --dry-run` |
| `userinfo` / `users` | `users list` |
| `users --json-pretty` | `users list -o json-pretty` |
| `sysinfo` | `banner -o json` |
| `banner` | `banner` (unchanged) |
| `cron status` | `schedule list` |
| `cron init` | `schedule enable update-cache` |
| `cron init --disable` | `schedule disable update-cache` |
| `cron updates --security` | `schedule enable updates --scope security` |
| `cron updates --disable` | `schedule disable updates` |
| `cron autoupdate` | `schedule enable self-update` |
| `cron kernels --keep 4` | `schedule enable kernel-cleanup --keep 4` |

### The one deliberate behaviour change

`kernel cleanup` currently previews unless given `--execute`. Under the proposal it acts
unless given `--dry-run`.

**Decision: accept it, and carry it as a release note.** The command keeps its name — no
rename to `kernel prune`.

What this means in practice, and what the release note has to say plainly: a caller that
passes `--execute` today keeps working (the flag is accepted and deprecated, and the command
was going to remove packages anyway). The exposed case is a caller that passes **no** flag
expecting a preview — it now removes packages. The known instance is an operator typing
`kernel cleanup` by hand to see what would go; the generated cron job already passes
`--execute` and is unaffected.

Draft entry for `CHANGELOG.md` under `### Changed`, worded so it cannot be skimmed past:

> - **BREAKING:** `syschecks kernel cleanup` now removes old kernel packages by default.
>   Previously it only previewed them unless `--execute` was given. Use
>   `syschecks kernel cleanup --dry-run` for the old preview behaviour. `--execute` is still
>   accepted but deprecated and has no effect beyond the new default. This aligns kernel
>   cleanup with every other mutating command, which now share a single `--dry-run`
>   convention.

Two supporting measures, since a release note alone is a weak guard for an inverted default:

- `--execute` stays accepted and deprecated (`MarkDeprecated`) rather than being removed, so
  existing invocations neither fail nor change meaning.
- When `kernel cleanup` runs with no flags on an interactive terminal, print the package list
  and require confirmation. Unattended runs (no TTY, as `updates apply` already detects) skip
  the prompt, so cron is unaffected. That turns the risky case — a human expecting a preview —
  into a prompt rather than a surprise.

---

## Migration

Renaming is cheap in the binary and expensive on deployed hosts, because the old strings are
written into files that live on those hosts:

- **Generated cron files** in `/etc/cron.d/syschecks_*` embed literal command lines
  (`syschecks apply-updates`, `syschecks updates --cache-create`,
  `syschecks kernel cleanup --execute --keep N`, `syschecks self-update`). A host that
  self-updates to a version with renamed commands keeps running the **old** strings until its
  cron files are regenerated.
- **Zabbix** installs `UserParameter=syschecks[*],syschecks $1`, so any item key an operator
  configured (`syschecks[kernel]`, `syschecks[updates]`, …) is a command name in disguise.

### Step 1 — aliases and deprecated flags (agreed)

Add the new tree with the old names kept as cobra `Aliases`, and old flags kept but hidden via
`Flags().MarkDeprecated("execute", "use --dry-run to preview instead")`. Nothing breaks on day
one, and `--help` stops advertising the old spellings.

The `userinfo` → `users` rename already landed and follows exactly this pattern
(`Aliases: []string{"userinfo"}`), so it is a working precedent.

### Step 2 — `syschecks migrate`, never an on-startup check

An automatic check at startup was rejected: `banner` runs on every SSH login, and even
100–200 ms of extra work is a delay every operator pays on every login, forever, to fix
something that needs doing once.

Instead, migration is an explicit command:

```
syschecks migrate            # report only: what would change, and in which file
syschecks migrate --apply    # actually rewrite
```

Scope:

- Scan `/etc/cron.d/syschecks_*` (and the legacy `automatic_*` job names) and rewrite embedded
  command strings to the new spellings.
- Scan the Zabbix agent config for `UserParameter` lines referencing renamed commands.
- Report every file it would touch, with a before/after line, and exit non-zero if anything is
  outstanding — so a monitoring check can ask "is this host fully migrated?".

Default is **report-only**. `--apply` is the opt-in, so installation and upgrade scripts must
name it deliberately:

```sh
# last line of install.sh / upgrade.sh
syschecks migrate --apply
```

Keeping it behind `--apply` means no unattended rewrite happens silently and the operator
running the installer knows what they are stepping into. Being written in Go rather than shell
means the installer gets it for free on every distro, with no duplicated `sed` logic.

Aliases from step 1 stay for at least one full release cycle after `migrate` ships, so a host
that has not run it yet still works.

---

## Decisions taken (review round 1)

| Question | Decision |
|---|---|
| `system banner` or keep `banner` top-level? | **Keep `banner` top-level.** Moving it would require editing a profile file on every managed host. |
| Keep `sysinfo`? | **Delete it.** Its content is reachable via `banner -o json`. |
| `updates cache refresh` or `updates refresh`? | **Both** — one is an alias of the other. |
| `schedule enable <job>` or `schedule <job> enable`? | **`schedule enable <job>`**, with `<job>` tab-completable. |
| On-startup migration check? | **No** — login latency. Explicit `syschecks migrate --apply` instead. |
| Global `--output`? | **Yes**, with shell completion on its values. |
| `kernel cleanup` `--execute` → `--dry-run` inversion? | **Accept it**, no rename. Release note + deprecated `--execute` + interactive confirmation. |
| Who installs the completion script? | **Already done** by `auto-install.sh` / `install-offline.sh`, and regenerated on upgrade. |

## Implementation notes

Everything in this document is now in the code. Two things worth recording:

- **`updates check` still defaults to the cache.** Problem 8 above criticised `--cache-use`
  defaulting to true, but flipping the default would have made every Zabbix poll run a full
  `makecache`/`apt-get update`. The staleness concern is already answered by the banner's
  update-cache check and by `cache_up_to_date` in the JSON, so the default stayed and
  `--refresh` was added for an explicit live query. Raise this if you want it flipped anyway.
- **`kernel cleanup` gained an interactive confirmation** beyond what the proposal specified,
  because a release note is a weak guard for an inverted default. Only `y`/`yes` proceeds;
  a blank line, EOF, or an unreadable stdin aborts. Cron has no TTY and is unaffected.

## Already implemented before this proposal

- `userinfo` → `users` with alias; `Active` column → `Logged in`.
- `schedule list` / `cron status` gained an **Enabled** column with a status symbol
  (`✅ yes` / `🟡 yes` legacy / `🛑 yes` conflict / `❌ no`), so on/off is readable at a glance
  instead of being inferred from six different `State` strings. The word carries the meaning
  on its own where emoji do not render.
