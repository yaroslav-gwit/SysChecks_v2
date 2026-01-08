# SysChecks Project Constitution

## Core Principles

1. **Linux-First, Linux-Only**: This tool is designed exclusively for Linux systems. Do not add support for macOS, Windows, or other operating systems.

2. **Root Privilege Awareness**: Clearly distinguish between commands that require root and those that don't. Always use `helpers.RootUserCheck()` for privileged operations.

3. **JSON Output by Default**: Most commands should output structured JSON for Zabbix integration. Offer `--json-pretty` flags for more human-readable output.

4. **Zero Breaking Changes**: Maintain backward compatibility with existing Zabbix templates and cron jobs. JSON schema changes must be additive only.

5. **Performance Over Features**: Prefer caching mechanisms over real-time queries. The tool may be called frequently by Zabbix (every minute).

## Code Style Conventions

### Go Language Standards

```go
// DO: Use gofmt formatting (automatic)
// DO: Use descriptive variable names
runningKernel := getRunningKernel()

// DON'T: Use single-letter variables except in loops
k := getRunningKernel()  // ❌
```

### Naming Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Commands | lowercase, single word or hyphenated | `kernel`, `apply-updates` |
| Variables | camelCase | `kernelJsonPretty` |
| Functions | camelCase, verb prefix | `getRunningKernel()` |
| Structs | camelCase + "Struct" suffix | `systemUpdatesStruct` |
| Constants | UPPER_SNAKE_CASE | `SECURITY_UPDATES_JOB` |

### Command Structure

**Every new Cobra command MUST follow this pattern:**

```go
var (
    // Flags declared here
    cmdFlagName bool
    
    cmdName = &cobra.Command{
        Use:   "command-name",
        Short: "One-line description",
        Long:  `Multi-line detailed description.`,
        Args:  cobra.NoArgs,  // Or other arg validator
        Run: func(cmd *cobra.Command, args []string) {
            cmdAction()
        },
        Aliases: []string{"alias1", "alias2"},  // Optional
    }
)

func cmdAction() {
    // Implementation in separate function for testability
}
```

**Registration in `cmd/root.go` init():**

```go
func init() {
    rootCmd.AddCommand(cmdName)
    cmdName.Flags().BoolVarP(&cmdFlagName, "flag", "f", false, "Description")
}
```

## Architectural Rules

### 1. Package Organization

```
DO:
✅ Command logic → cmd/ package
✅ Shared utilities → helpers/ package
✅ Templates/constants → helpers/ package

DON'T:
❌ Business logic in main.go
❌ OS detection in command files (use helpers)
❌ Duplicate code across commands
```

### 2. Error Handling

```go
// DO: Fatal errors for unrecoverable situations
if err != nil {
    log.Fatal("Clear description: " + err.Error())
}

// DO: Check exit codes when they matter
if cmd.ProcessState.ExitCode() == 100 {
    // DNF returns 100 when updates are available
}

// DON'T: Silent failures
if err != nil {
    _ = err  // ❌ Never ignore errors silently
}

// DON'T: Panic in normal code paths
panic("something went wrong")  // ❌ Use log.Fatal instead
```

### 3. OS Detection Pattern

```go
// DO: Use the standard detectOs() function
osType := detectOs()
if osType.deb {
    // Debian/Ubuntu logic
} else if osType.dnf {
    // RHEL 8+/AlmaLinux/Rocky logic
} else if osType.yum {
    // CentOS logic
}

// DON'T: Check OS in multiple places
if strings.Contains(osRelease, "ubuntu") {  // ❌ Don't repeat OS detection
```

### 4. JSON Output Pattern

```go
// DO: Marshal to JSON with optional pretty printing
type OutputStruct struct {
    Field string `json:"field_name"`  // snake_case in JSON
}

if prettyFlag {
    jsonOut, _ := json.MarshalIndent(data, "", "   ")
    fmt.Println(string(jsonOut))
} else {
    jsonOut, _ := json.Marshal(data)
    fmt.Println(string(jsonOut))
}

// DON'T: Mix output formats
fmt.Println("Status: OK")  // ❌ If command outputs JSON, always output JSON
fmt.Println(jsonString)
```

## Security Guidelines

### 1. Root Privilege Management

```go
// DO: Check root access at function start
func privilegedAction() {
    helpers.RootUserCheck()  // Exits if not root
    // ... privileged operations
}

// DO: Document root requirement in command Long description
Long: `This command requires root privileges to modify system files.`

// DON'T: Perform privileged operations without checking
os.WriteFile("/etc/important", data, 0644)  // ❌ Check root first
```

### 2. File Permissions

```go
// DO: Use appropriate permissions
const CRON_FILE_PERMS = 0644  // Read/write owner, read others

// DO: Explicitly set permissions after creation
os.WriteFile(path, data, 0644)
exec.Command("chmod", "0644", path).Run()  // Extra safety for hardened systems

// DON'T: Use overly permissive modes
os.WriteFile(path, data, 0777)  // ❌ Too permissive
```

### 3. Command Execution

```go
// DO: Use CommandContext with timeout for long operations
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()
cmd := exec.CommandContext(ctx, "apt", "update")

// DO: Validate/sanitize inputs if accepting user data
// (Currently not needed as we don't take arbitrary user input)

// DON'T: Use shell=True equivalents
exec.Command("bash", "-c", userInput)  // ❌ Shell injection risk
```

## Performance Guidelines

### 1. Caching Strategy

```go
// DO: Implement caching for expensive operations
// - Update checks should use /tmp/syscheck_updates.json
// - Offer --cache-create and --cache-use flags

// DO: Set reasonable TTL via cron
// Cache updates every 12 hours is sufficient for most use cases

// DON'T: Run expensive operations on every Zabbix query
// DNF/APT cache refresh is SLOW - always cache results
```

### 2. Regex Compilation

```go
// DO: Compile regex patterns once
reMatch := regexp.MustCompile(`^Pattern.*`)

// DON'T: Compile in loops
for _, item := range items {
    re := regexp.MustCompile(`pattern`)  // ❌ Compile outside loop
}
```

### 3. System Command Optimization

```go
// DO: Use specific commands
exec.Command("lscpu")  // Fast, targeted

// DON'T: Parse entire files when commands exist
// Prefer lscpu over parsing /proc/cpuinfo manually
```

## Testing Requirements

### Manual Testing Checklist

Before committing changes, test:

1. ✅ Build succeeds: `./build.sh`
2. ✅ Command executes: `./bin/syschecks <your-command>`
3. ✅ JSON validates (if applicable): `./bin/syschecks <cmd> | jq`
4. ✅ Root check works (if applicable): Try as non-root user
5. ✅ Help text renders: `./bin/syschecks <cmd> --help`
6. ✅ Flags work as expected

### Test on Multiple Distros

If modifying OS-specific logic, test on:
- Ubuntu/Debian (apt)
- AlmaLinux/Rocky (dnf)
- CentOS 7 (yum) - if still supported

### Don't Break Zabbix

If modifying JSON output structure:
```bash
# Compare old vs new output
./bin/syschecks-old kernel > old.json
./bin/syschecks-new kernel > new.json
diff old.json new.json
# Ensure existing fields remain unchanged
```

## Documentation Standards

### Command Documentation

```go
var cmd = &cobra.Command{
    Use:   "command [flags]",
    Short: "One line, no period, under 60 chars",
    Long: `Multi-line description explaining:
- What the command does
- When to use it
- Any prerequisites (root, etc.)
- Output format if applicable`,
}
```

### Function Comments

```go
// DO: Comment exported functions and complex logic
// GetRunningKernel returns the currently running kernel version
// by executing 'uname -r' and cleaning up the output.
func GetRunningKernel() string {

// DO: Explain non-obvious regex patterns
// Match vmlinuz-* files, but skip rescue kernels
reMatch := regexp.MustCompile(`vmlinuz-.*`)
reIgnore := regexp.MustCompile(`.*0-rescue.*`)

// DON'T: State the obvious
// This function adds two numbers  // ❌
func Add(a, b int) int {
```

### Inline Comments

```go
// DO: Explain "why", not "what"
// DNF returns exit code 100 when updates are available (not an error)
if cmd.ProcessState.ExitCode() == 100 {

// DON'T: Redundant comments
i++ // increment i  // ❌
```

## Commit Message Standards

```
<type>: <subject>

<body>

<footer>
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code restructuring without behavior change
- `docs`: Documentation only
- `perf`: Performance improvement
- `test`: Adding tests

### Examples

```
feat: add automatic package cleanup for old kernels

Implement 'kernel cleanup' subcommand that generates removal
commands for kernels older than the specified --keep threshold.

Closes #42
```

```
fix: handle DNF exit code 100 correctly

DNF returns exit code 100 when updates are available, which
is not an error condition. Updated error handling to treat
this as success.
```

## What to Avoid

### ❌ Avoid These Patterns

1. **Adding Windows/macOS support** - This is a Linux-only tool
2. **Breaking JSON schema** - Zabbix templates depend on existing structure
3. **Removing commands** - Maintain backward compatibility
4. **Complex flag combinations** - Keep UX simple
5. **Interactive prompts** - Tool must work in cron/Zabbix contexts
6. **Logging to stdout** - Use stderr for logs, stdout for data
7. **Global state** - Keep functions pure when possible
8. **External dependencies** - Minimize new Go module dependencies
9. **Hardcoded paths** - Use constants for file locations
10. **Magic numbers** - Define constants for timeouts, limits, etc.

### ✅ Embrace These Patterns

1. **Idempotent operations** - Commands should be safe to run repeatedly
2. **Defensive programming** - Check file existence before reading
3. **Graceful degradation** - Handle missing optional features
4. **Clear error messages** - Include context and remediation steps
5. **Minimal output** - JSON only for data commands
6. **Semantic versioning** - Use build.sh for version management
7. **Cache-friendly** - Design for frequent execution
8. **Single responsibility** - Each command does one thing well

## Version Compatibility

### Build Time

```bash
# ALWAYS use build.sh for proper version injection
./build.sh

# DON'T build without version info
go build  # ❌ Missing version metadata
```

### Breaking Changes

**If you must make breaking changes:**

1. Bump major version (1.x.x → 2.0.0)
2. Document migration path in RELEASES.md
3. Provide conversion script if needed
4. Announce deprecation at least one version early

## Code Review Checklist

Before submitting changes:

- [ ] Follows naming conventions
- [ ] Uses `helpers.RootUserCheck()` if needed
- [ ] JSON output is backward compatible
- [ ] Help text is clear and accurate
- [ ] Command is registered in `root.go`
- [ ] No hardcoded paths or magic numbers
- [ ] Error messages are descriptive
- [ ] Manual testing completed
- [ ] No new external dependencies without discussion
- [ ] Documentation updated (if user-facing change)

## Philosophy

> "Do one thing and do it well. Output structured data for automation. Cache aggressively. Never break backward compatibility. Be fast, be reliable, be boring."

This tool is infrastructure. Stability > Features. Performance > Convenience. Automation > Interaction.
