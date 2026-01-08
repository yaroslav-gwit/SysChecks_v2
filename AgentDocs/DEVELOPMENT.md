# SysChecks Development Guide

## Prerequisites

- Go 1.21 or later
- Linux system (for testing) or Docker
- Git

## Setting Up Development Environment

```bash
# Clone the repository
git clone <repo-url>
cd SysChecks_v2

# Download dependencies
go mod download

# Build
./build.sh

# Or build manually
go build -o ./bin/syschecks
```

## Build Options

### Standard Build

```bash
./build.sh
```

Creates `./bin/syschecks` with version info from git.

### Advanced Build (Cross-Compilation)

```bash
./build-advanced.sh
```

### Docker Build (Portable Binary)

```bash
./docker-build.sh
```

Uses Ubuntu 18.04 base for maximum glibc compatibility. All Docker-related files are in the `docker/` directory.

### Manual Build with Version Info

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

go build -ldflags="-X 'syschecks/cmd.Version=$VERSION' \
  -X 'syschecks/cmd.GitCommit=$COMMIT' \
  -X 'syschecks/cmd.BuildDate=$DATE'" \
  -o ./bin/syschecks
```

## Adding a New Command

### 1. Create Command File

Create `cmd/newcmd.go`:

```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var (
    // Flags
    newCmdFlag bool

    newCmd = &cobra.Command{
        Use:   "newcmd",
        Short: "Brief description",
        Long:  `Detailed description of what the command does.`,
        Run: func(cmd *cobra.Command, args []string) {
            newCmdAction()
        },
    }
)

func newCmdAction() {
    fmt.Println("New command executed")
}
```

### 2. Register in root.go

Add to the `init()` function in `cmd/root.go`:

```go
func init() {
    // ... existing commands ...
    
    rootCmd.AddCommand(newCmd)
    newCmd.Flags().BoolVarP(&newCmdFlag, "flag", "f", false, "Flag description")
}
```

### 3. For Subcommands

```go
// In cmd/parent.go
parentCmd.AddCommand(newSubCmd)
```

## Adding Helper Functions

Add to `helpers/generalHelpers.go` or create a new file:

```go
package helpers

func NewHelper() string {
    // Implementation
    return "result"
}
```

## Code Patterns

### JSON Output

```go
import "encoding/json"

type OutputStruct struct {
    Field1 string `json:"field_1"`
    Field2 int    `json:"field_2"`
}

func outputJSON(pretty bool) {
    data := OutputStruct{Field1: "value", Field2: 42}
    
    if pretty {
        jsonOut, _ := json.MarshalIndent(data, "", "   ")
        fmt.Println(string(jsonOut))
    } else {
        jsonOut, _ := json.Marshal(data)
        fmt.Println(string(jsonOut))
    }
}
```

### Root User Check

```go
import "syschecks/helpers"

func actionRequiringRoot() {
    helpers.RootUserCheck()  // Exits if not root
    // ... rest of implementation
}
```

### OS Detection

```go
func doSomething() {
    osType := detectOs()
    
    if osType.deb {
        // Debian/Ubuntu specific
    } else if osType.dnf {
        // RHEL 8+ specific
    } else if osType.yum {
        // CentOS specific
    }
}
```

### Running System Commands

```go
import "os/exec"

func runCommand() {
    cmd := exec.Command("command", "arg1", "arg2")
    stdout, err := cmd.Output()
    if err != nil {
        log.Fatal(err)
    }
    result := strings.TrimSpace(string(stdout))
}
```

## Testing

### Manual Testing on Linux

```bash
# Build and test
./build.sh
./bin/syschecks version -v
./bin/syschecks kernel --json-pretty
sudo ./bin/syschecks updates --cache-create
./bin/syschecks banner
```

### Testing in Docker

```bash
# Build using Docker
docker build -t syschecks-build .

# Run in a container
docker run --rm -it ubuntu:22.04 bash
# Then copy and test the binary inside
```

## Debugging

### Verbose Output

Add temporary debug logging:

```go
import "log"

log.Printf("Debug: value = %v\n", variable)
```

### Check Exit Codes

```bash
./bin/syschecks updates; echo "Exit code: $?"
```

## Release Process

### Using release.sh

```bash
# Create a release
./release.sh 1.0.0

# Create a prerelease
./release.sh 1.0.0-beta prerelease

# Create a draft
./release.sh 1.0.0 draft
```

### Manual Tag and Release

```bash
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions will create the release
```

## Common Issues

### "Not running as root"

Many commands require root. Run with `sudo`:

```bash
sudo ./bin/syschecks updates --cache-create
```

### "OS not supported"

The tool only supports:
- Ubuntu, Debian, Pop!_OS
- CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux

### Build Errors

```bash
# Clean and rebuild
rm -rf ./bin/
go clean -cache
go mod tidy
./build.sh
```

## File Locations Summary

| Development | Production |
|-------------|------------|
| `./bin/syschecks` | `/bin/syschecks` |
| `./package.lock.json` | `/opt/syschecks/package.lock.json` |
| - | `/etc/cron.d/syschecks_*` |
| - | `/tmp/syscheck_updates.json` |
