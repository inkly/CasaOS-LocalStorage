package model

import "strings"

// SmartHealth values returned by SmartctlA.SmartHealth.
const (
	SmartHealthPassed      = "passed"
	SmartHealthFailed      = "failed"
	SmartHealthUnavailable = "unavailable"
)

// smartctl exit status bits, see smartctl(8) EXIT STATUS.
const (
	smartctlExitDeviceOpenFailed = 1 << 1
	smartctlExitDiskFailing      = 1 << 3
)

type SmartStatus struct {
	Passed bool `json:"passed"`
}

type SmartctlA struct {
	Smartctl struct {
		Version      []int    `json:"version"`
		SvnRevision  string   `json:"svn_revision"`
		PlatformInfo string   `json:"platform_info"`
		BuildInfo    string   `json:"build_info"`
		Argv         []string `json:"argv"`
		ExitStatus   int      `json:"exit_status"`
		Messages     []struct {
			String   string `json:"string"`
			Severity string `json:"severity"`
		} `json:"messages"`
	} `json:"smartctl"`
	Device struct {
		Name     string `json:"name"`
		InfoName string `json:"info_name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelName       string `json:"model_name"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	UserCapacity    struct {
		Blocks int   `json:"blocks"`
		Bytes  int64 `json:"bytes"`
	} `json:"user_capacity"`
	SmartStatus  *SmartStatus `json:"smart_status"`
	AtaSmartData struct {
		OfflineDataCollection struct {
			Status struct {
				Value  int    `json:"value"`
				String string `json:"string"`
			} `json:"status"`
			CompletionSeconds int `json:"completion_seconds"`
		} `json:"offline_data_collection"`
		SelfTest struct {
			Status struct {
				Value  int    `json:"value"`
				String string `json:"string"`
				Passed bool   `json:"passed"`
			} `json:"status"`
			PollingMinutes struct {
				Short      int `json:"short"`
				Extended   int `json:"extended"`
				Conveyance int `json:"conveyance"`
			} `json:"polling_minutes"`
		} `json:"self_test"`
		Capabilities struct {
			Values                        []int `json:"values"`
			ExecOfflineImmediateSupported bool  `json:"exec_offline_immediate_supported"`
			OfflineIsAbortedUponNewCmd    bool  `json:"offline_is_aborted_upon_new_cmd"`
			OfflineSurfaceScanSupported   bool  `json:"offline_surface_scan_supported"`
			SelfTestsSupported            bool  `json:"self_tests_supported"`
			ConveyanceSelfTestSupported   bool  `json:"conveyance_self_test_supported"`
			SelectiveSelfTestSupported    bool  `json:"selective_self_test_supported"`
			AttributeAutosaveEnabled      bool  `json:"attribute_autosave_enabled"`
			ErrorLoggingSupported         bool  `json:"error_logging_supported"`
			GpLoggingSupported            bool  `json:"gp_logging_supported"`
		} `json:"capabilities"`
	} `json:"ata_smart_data"`
	PowerOnTime struct {
		Hours int `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount int `json:"power_cycle_count"`
	Temperature     struct {
		Current int `json:"current"`
	} `json:"temperature"`
}

// SmartHealth reduces a smartctl report to "passed", "failed" or "unavailable".
// Virtual disks (QEMU, Hyper-V) return no smart_status object at all, which
// must not be read as a failure.
func (m SmartctlA) SmartHealth() string {
	if (m.SmartStatus != nil && !m.SmartStatus.Passed) || m.Smartctl.ExitStatus&smartctlExitDiskFailing != 0 {
		return SmartHealthFailed
	}
	if m.SmartStatus == nil || m.Smartctl.ExitStatus&smartctlExitDeviceOpenFailed != 0 {
		return SmartHealthUnavailable
	}
	for _, msg := range m.Smartctl.Messages {
		if strings.Contains(msg.String, "STANDBY") {
			return SmartHealthUnavailable
		}
	}
	return SmartHealthPassed
}

// AggregateSmartHealth summarises several SmartHealth values: "failed" if any
// disk failed, "passed" if at least one passed, "unavailable" otherwise.
func AggregateSmartHealth(healths ...string) string {
	result := SmartHealthUnavailable
	for _, h := range healths {
		switch h {
		case SmartHealthFailed:
			return SmartHealthFailed
		case SmartHealthPassed:
			result = SmartHealthPassed
		}
	}
	return result
}
