# Zabbix Integration Guide

## Overview

SysChecks integrates with Zabbix monitoring to provide system health metrics through custom UserParameters.

## Quick Setup

```bash
# Initialize Zabbix integration
sudo syschecks zabbix init

# Set up update cache cron job
sudo syschecks cron init
```

## How It Works

### UserParameter Configuration

The `zabbix init` command adds the following line to your Zabbix agent configuration:

```ini
#_ SYSCHECKS INTEGRATION _#
UserParameter=syschecks[*],syschecks $1
```

Because the key is passed straight through as the subcommand, an item key **is** a command
name. The v1.3.0 restructure keeps every previous spelling working, so existing items such as
`syschecks[kernel]` and `syschecks[updates]` continue to resolve. `syschecks migrate --apply`
rewrites any explicit (non-wildcard) `UserParameter` lines that name a retired command.

### Recommended keys after v1.3.0

| Key | Reports |
|-----|---------|
| `syschecks[banner]` with `-o json`* | Everything below in one item, including a `healthy` boolean per check |
| `syschecks[kernel]` | Reboot required, running/latest kernel, installed count |
| `syschecks[updates]` | System and security update counts, plus `repository_issues` |

\* For a single-item setup, define a dedicated parameter rather than relying on the wildcard:
`UserParameter=syschecks.banner,syschecks banner --output json`. It reports every check —
including healthy ones the human banner hides — so a trigger can distinguish "check passed"
from "check missing".

Repository health is new in v1.3.0 and worth a trigger: a repository that has lost its GPG
key or disappeared makes update counts silently understated.
`$.repository_issue_count > 0` catches it.


This allows Zabbix to call syschecks commands with parameters.

### Zabbix Agent Config Locations

The tool checks these locations (in order):
1. `/etc/zabbix/zabbix_agentd.conf`
2. `/etc/zabbix_agentd.conf`

## Available Metrics

### Kernel Checks

**Item Key:** `syschecks[kernel]`

**Returns:** JSON with kernel status

```json
{
  "kernel_needs_reboot": true,
  "running_kernel": "5.15.0-91",
  "latest_installed_kernel": "5.15.0-92",
  "active_kernels": ["5.15.0-91", "5.15.0-92"]
}
```

**Zabbix Template Item:**
```
Type: Zabbix agent
Key: syschecks[kernel]
Type of information: Text
```

**Dependent Items:**
- `kernel_needs_reboot` - Boolean trigger for reboot required
- `running_kernel` - Current kernel version
- `latest_installed_kernel` - Newest installed kernel

### Update Checks

**Item Key:** `syschecks[updates]`

**Returns:** JSON with update information

```json
{
  "number_of_system_updates": 15,
  "number_of_security_updates": 3,
  "system_updates_available": true,
  "security_updates_available": true,
  "system_updates_list": [...],
  "security_updates_list": [...]
}
```

**Zabbix Template Item:**
```
Type: Zabbix agent
Key: syschecks[updates]
Type of information: Text
```

**Dependent Items:**
- `number_of_security_updates` - Count for alerting
- `security_updates_available` - Boolean trigger

### System Info

**Item Key:** `syschecks[banner]` (formerly `syschecks[banner]`)

**Returns:** JSON with IP addresses

```json
{"ip_address_list": "192.168.1.100, 10.0.0.1"}
```

## Cache Configuration

For faster Zabbix queries, set up the update cache:

```bash
# Create cron job for cache updates
sudo syschecks cron init
```

This creates `/etc/cron.d/syschecks_cache`:
- Runs on boot (with random delay)
- Runs every 12 hours
- Stores cache at `/tmp/syscheck_updates.json`

## Sample Zabbix Template

### Template Structure

```yaml
Template: Template SysChecks
  Items:
    - Name: Kernel Status
      Key: syschecks[kernel]
      Type: Zabbix agent
      Update interval: 1h
      
    - Name: Update Status
      Key: syschecks[updates]
      Type: Zabbix agent
      Update interval: 6h
      
  Triggers:
    - Name: Kernel reboot required on {HOST.NAME}
      Expression: {Template SysChecks:syschecks[kernel].jsonpath("$.kernel_needs_reboot")}=1
      Severity: Warning
      
    - Name: Security updates available on {HOST.NAME}
      Expression: {Template SysChecks:syschecks[updates].jsonpath("$.number_of_security_updates")}>0
      Severity: Information
```

### JSONPath Preprocessing

Use Zabbix's JSONPath preprocessing to extract specific values:

| Master Item | Preprocessing | Result |
|-------------|---------------|--------|
| `syschecks[kernel]` | `$.kernel_needs_reboot` | `true`/`false` |
| `syschecks[kernel]` | `$.running_kernel` | `"5.15.0-91"` |
| `syschecks[updates]` | `$.number_of_security_updates` | `3` |

## Troubleshooting

### Test from Command Line

```bash
# As zabbix user or root
syschecks kernel
syschecks updates
```

### Check Zabbix Agent Config

```bash
# Verify UserParameter was added
grep -i syschecks /etc/zabbix/zabbix_agentd.conf
```

### Test via zabbix_get

```bash
# From Zabbix server
zabbix_get -s <host> -k "syschecks[kernel]"
```

### Common Issues

**Issue:** Empty or error response
**Solution:** Ensure cache is created:
```bash
sudo syschecks updates --cache-create
```

**Issue:** Permission denied
**Solution:** The `zabbix` user should be able to read the cache file:
```bash
ls -la /tmp/syscheck_updates.json
# Should be readable by all users
```

**Issue:** Command not found
**Solution:** Ensure binary is in PATH:
```bash
which syschecks
# Should return /bin/syschecks
```

## Best Practices

1. **Use caching**: Always set up `cron init` for production systems
2. **Reasonable intervals**: Don't query updates too frequently (every 6h is fine)
3. **Alert on security updates**: Set up triggers for `number_of_security_updates > 0`
4. **Monitor kernel status**: Create triggers for `kernel_needs_reboot = true`
5. **Log file monitoring**: Consider monitoring `/var/log/syschecks_updates.log` if using automatic updates
