package cmd

import (
	"fmt"
	"syschecks/helpers"
)

// The JSON view of the banner. It deliberately reports *every* check, including the healthy
// ones the human banner suppresses, because a monitoring system has to be able to tell
// "this check passed" apart from "this check is missing". It also absorbs everything the
// retired `sysinfo` command used to print, so no command had to move out of the top level
// and no /etc/profile.d file on a deployed host has to change.

type bannerCheck struct {
	Healthy bool   `json:"healthy"`
	Detail  string `json:"detail,omitempty"`
}

type bannerChecks struct {
	KernelReboot     bannerCheck `json:"kernel_reboot"`
	InstalledKernels bannerCheck `json:"installed_kernels"`
	DiskSpace        bannerCheck `json:"disk_space"`
	AutomaticUpdates bannerCheck `json:"automatic_updates"`
	SystemUpdates    bannerCheck `json:"system_updates"`
	SecurityUpdates  bannerCheck `json:"security_updates"`
	Repositories     bannerCheck `json:"repositories"`
	UpdateCache      bannerCheck `json:"update_cache"`
}

type bannerRAM struct {
	Used  string `json:"used"`
	Total string `json:"total"`
}

type bannerDisk struct {
	MountPoint     string  `json:"mount_point"`
	Source         string  `json:"source"`
	Filesystem     string  `json:"filesystem"`
	FreePercent    float64 `json:"free_percent"`
	AvailableBytes uint64  `json:"available_bytes"`
	Available      string  `json:"available"`
	ReadOnly       bool    `json:"read_only"`
}

type bannerJSONStruct struct {
	Hostname    string   `json:"hostname"`
	OS          string   `json:"os"`
	Uptime      string   `json:"uptime"`
	UptimeSecs  float64  `json:"uptime_seconds"`
	IPAddresses []string `json:"ip_addresses"`
	// IPAddressList preserves the exact field the retired `sysinfo` command emitted, so
	// Zabbix items keyed on it keep resolving after `sysinfo` goes away.
	IPAddressList     string                  `json:"ip_address_list"`
	CPU               string                  `json:"cpu"`
	RAM               bannerRAM               `json:"ram"`
	LoggedInUsers     int                     `json:"logged_in_users"`
	LoginSessions     int                     `json:"login_sessions"`
	Disks             []bannerDisk            `json:"disks"`
	Kernel            kernelJsonOutputStruct  `json:"kernel"`
	Updates           systemUpdatesJsonStruct `json:"updates"`
	AutomaticUpdates  string                  `json:"automatic_updates"`
	SelfUpdateEnabled bool                    `json:"self_update_enabled"`
	Version           string                  `json:"version"`
	Healthy           bool                    `json:"healthy"`
	Checks            bannerChecks            `json:"checks"`
}

func collectBannerData(diskUsedThreshold float64) bannerJSONStruct {
	freeThreshold := 100 - diskUsedThreshold

	sessions := readLoginSessions()
	sessionCount := 0
	for _, entries := range sessions {
		sessionCount += len(entries)
	}

	ramInfo := helpers.GetRamInfoLinux()
	kernComp := compareKernels()
	sysUpdates := systemUpdates(true)
	updateMode := currentAutomaticOSUpdateMode()

	disks := make([]bannerDisk, 0)
	lowDisks := make([]string, 0)
	for _, disk := range diskSpacePartitions() {
		disks = append(disks, bannerDisk{
			MountPoint:     disk.mountPoint,
			Source:         disk.source,
			Filesystem:     disk.filesystem,
			FreePercent:    disk.freePercent,
			AvailableBytes: disk.availableBytes,
			Available:      formatBytes(disk.availableBytes),
			ReadOnly:       disk.readOnly,
		})
		if !disk.readOnly && disk.freePercent < freeThreshold {
			lowDisks = append(lowDisks, fmt.Sprintf("%s at %.1f%% free", disk.mountPoint, disk.freePercent))
		}
	}

	data := bannerJSONStruct{
		Hostname:          getHostName(),
		OS:                helpers.PrettyOsName(),
		Uptime:            getSystemUptime(),
		UptimeSecs:        getSystemUptimeSeconds(),
		IPAddresses:       getIpList(),
		IPAddressList:     getIps(),
		CPU:               helpers.GetCpuInfoLinux(),
		RAM:               bannerRAM{Used: ramInfo.Used, Total: ramInfo.Total},
		LoggedInUsers:     len(sessions),
		LoginSessions:     sessionCount,
		Disks:             disks,
		Kernel:            kernelJsonOutput(),
		Updates:           sysUpdates,
		AutomaticUpdates:  string(updateMode),
		SelfUpdateEnabled: selfUpdateEnabled(helpers.AUTOUPDATE_JOB),
		Version:           GetVersion(),
	}

	data.Checks = bannerChecks{
		KernelReboot: bannerCheck{
			Healthy: !kernComp.kernelNeedsReboot,
			Detail:  kernelRebootDetail(kernComp),
		},
		InstalledKernels: bannerCheck{
			Healthy: kernComp.installedKernelCount <= kernelCleanupThreshold,
			Detail:  fmt.Sprintf("%d installed", kernComp.installedKernelCount),
		},
		DiskSpace: bannerCheck{
			Healthy: len(lowDisks) == 0,
			Detail:  joinDetail(lowDisks),
		},
		AutomaticUpdates: bannerCheck{
			Healthy: updateMode == automaticOSUpdatesSecurity || updateMode == automaticOSUpdatesSystem,
			Detail:  string(updateMode),
		},
		SystemUpdates: bannerCheck{
			Healthy: sysUpdates.NumberOfSystemUpdates == 0,
			Detail:  fmt.Sprintf("%d available", sysUpdates.NumberOfSystemUpdates),
		},
		SecurityUpdates: bannerCheck{
			Healthy: sysUpdates.NumberOfSecurityUpdates == 0,
			Detail:  fmt.Sprintf("%d available", sysUpdates.NumberOfSecurityUpdates),
		},
		Repositories: bannerCheck{
			Healthy: len(sysUpdates.RepositoryIssues) == 0,
			Detail:  repositoryIssueDetail(sysUpdates.RepositoryIssues),
		},
		UpdateCache: bannerCheck{
			Healthy: sysUpdates.CacheUpToDate,
			Detail:  updateCacheDetail(sysUpdates),
		},
	}

	data.Healthy = allChecksHealthy(data.Checks)
	return data
}

// allChecksHealthy is deliberately explicit rather than reflective: adding a check should be
// a compile-time decision about whether it belongs in the overall verdict.
func allChecksHealthy(checks bannerChecks) bool {
	for _, check := range []bannerCheck{
		checks.KernelReboot,
		checks.InstalledKernels,
		checks.DiskSpace,
		checks.AutomaticUpdates,
		checks.SystemUpdates,
		checks.SecurityUpdates,
		checks.Repositories,
		checks.UpdateCache,
	} {
		if !check.Healthy {
			return false
		}
	}
	return true
}

func kernelRebootDetail(kernComp compareKernelsStruct) string {
	if !kernComp.kernelNeedsReboot {
		return "running " + kernComp.runningKernel
	}
	return "running " + kernComp.runningKernel + ", latest installed " + kernComp.latestInstalledKernel
}

func repositoryIssueDetail(issues []repoIssue) string {
	if len(issues) == 0 {
		return "all repositories refreshed"
	}
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Repo+": "+issue.Reason)
	}
	return joinDetail(parts)
}

func updateCacheDetail(sysUpdates systemUpdatesJsonStruct) string {
	switch {
	case !sysUpdates.CacheExists:
		return "no cache file"
	case !sysUpdates.CacheUpToDate:
		return "stale (created " + sysUpdates.CacheDateCreated + ")"
	default:
		return "created " + sysUpdates.CacheDateCreated
	}
}

func joinDetail(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "; "
		}
		result += part
	}
	return result
}
