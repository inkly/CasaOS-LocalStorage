package model

import (
	"encoding/json"
	"testing"
)

const smartctlSATAPassed = `{"json_format_version":[1,0],"smartctl":{"version":[7,3],"svn_revision":"5338","platform_info":"x86_64-linux-6.1.0-18-amd64","build_info":"(local build)","argv":["smartctl","-a","-n","standby","/dev/sda","-j"],"exit_status":0},"device":{"name":"/dev/sda","info_name":"/dev/sda [SAT]","type":"sat","protocol":"ATA"},"model_family":"Western Digital Red","model_name":"WDC WD40EFRX-68N32N0","serial_number":"WD-WCC7K1234567","firmware_version":"82.00A82","user_capacity":{"blocks":7814037168,"bytes":4000787030016},"smart_status":{"passed":true},"power_on_time":{"hours":21345},"power_cycle_count":57,"temperature":{"current":34}}`

const smartctlSATAFailed = `{"json_format_version":[1,0],"smartctl":{"version":[7,3],"svn_revision":"5338","platform_info":"x86_64-linux-6.1.0-18-amd64","build_info":"(local build)","argv":["smartctl","-a","-n","standby","/dev/sdb","-j"],"exit_status":8},"device":{"name":"/dev/sdb","info_name":"/dev/sdb [SAT]","type":"sat","protocol":"ATA"},"model_name":"ST3000DM001-1CH166","serial_number":"Z1F3ABCD","firmware_version":"CC29","user_capacity":{"blocks":5860533168,"bytes":3000592982016},"smart_status":{"passed":false},"power_on_time":{"hours":48211},"power_cycle_count":122,"temperature":{"current":41}}`

// smartctl 7.x on a Proxmox/QEMU virtual disk: no smart_status object at all.
const smartctlQEMU = `{"json_format_version":[1,0],"smartctl":{"version":[7,3],"svn_revision":"5338","platform_info":"x86_64-linux-6.8.12-4-pve","build_info":"(local build)","argv":["smartctl","-a","-n","standby","/dev/sda","-j"],"exit_status":4,"messages":[{"string":"Device does not support SMART","severity":"warning"}]},"device":{"name":"/dev/sda","info_name":"/dev/sda","type":"scsi","protocol":"SCSI"},"vendor":"QEMU","product":"QEMU HARDDISK","model_name":"QEMU HARDDISK","revision":"2.5+","scsi_version":"SPC-3","user_capacity":{"blocks":25165824,"bytes":12884901888},"logical_block_size":512,"device_type":{"scsi_terminology":"Peripheral Device Type [PDT]","scsi_value":0,"name":"disk"},"local_time":{"time_t":1757145600,"asctime":"Sat Sep  6 10:00:00 2026 CEST"},"temperature":{"current":0}}`

const smartctlOpenFailed = `{"json_format_version":[1,0],"smartctl":{"version":[7,3],"svn_revision":"5338","platform_info":"x86_64-linux-6.1.0-18-amd64","build_info":"(local build)","argv":["smartctl","-a","-n","standby","/dev/sdc","-j"],"exit_status":2,"messages":[{"string":"Smartctl open device: /dev/sdc failed: No such device","severity":"error"}]}}`

const smartctlStandby = `{"json_format_version":[1,0],"smartctl":{"version":[7,3],"svn_revision":"5338","platform_info":"x86_64-linux-6.1.0-18-amd64","build_info":"(local build)","argv":["smartctl","-a","-n","standby","/dev/sdd","-j"],"exit_status":2,"messages":[{"string":"Device is in STANDBY mode, exit(2)","severity":"info"}]},"device":{"name":"/dev/sdd","info_name":"/dev/sdd [SAT]","type":"sat","protocol":"ATA"},"model_name":"WDC WD40EFRX-68N32N0","serial_number":"WD-WCC7K7654321","firmware_version":"82.00A82"}`

func TestSmartHealth(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		{"sata passed", smartctlSATAPassed, SmartHealthPassed},
		{"sata failed", smartctlSATAFailed, SmartHealthFailed},
		{"qemu virtual disk without smart_status", smartctlQEMU, SmartHealthUnavailable},
		{"empty struct (smartctl missing or no output)", "{}", SmartHealthUnavailable},
		{"device open failed", smartctlOpenFailed, SmartHealthUnavailable},
		{"standby", smartctlStandby, SmartHealthUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m SmartctlA
			if err := json.Unmarshal([]byte(c.raw), &m); err != nil {
				t.Fatal(err)
			}
			if got := m.SmartHealth(); got != c.want {
				t.Fatalf("SmartHealth() = %q, want %q", got, c.want)
			}
		})
	}
	if got := (SmartctlA{}).SmartHealth(); got != SmartHealthUnavailable {
		t.Fatalf("zero value SmartHealth() = %q, want %q", got, SmartHealthUnavailable)
	}
}

func TestAggregateSmartHealth(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"no disks", nil, SmartHealthUnavailable},
		{"only unavailable (VM)", []string{SmartHealthUnavailable}, SmartHealthUnavailable},
		{"passed and unavailable", []string{SmartHealthUnavailable, SmartHealthPassed}, SmartHealthPassed},
		{"one failed among passed", []string{SmartHealthPassed, SmartHealthFailed, SmartHealthPassed}, SmartHealthFailed},
		{"failed and unavailable", []string{SmartHealthUnavailable, SmartHealthFailed}, SmartHealthFailed},
	}
	for _, c := range cases {
		if got := AggregateSmartHealth(c.in...); got != c.want {
			t.Errorf("%s: AggregateSmartHealth(%v) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
