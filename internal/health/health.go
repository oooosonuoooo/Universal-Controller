package health

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type Report struct {
	Hostname    string
	Platform    string
	Uptime      string
	CPUPercent  float64
	RAMPercent  float64
	DiskPercent float64
}

func Collect() (Report, error) {
	hostInfo, err := host.Info()
	if err != nil {
		return Report{}, err
	}
	cpuPercent, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil {
		return Report{}, err
	}
	vm, err := mem.VirtualMemory()
	if err != nil {
		return Report{}, err
	}
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	usage, err := disk.Usage(diskPath)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Hostname:    hostInfo.Hostname,
		Platform:    fmt.Sprintf("%s %s", hostInfo.Platform, hostInfo.PlatformVersion),
		Uptime:      humanDuration(time.Duration(hostInfo.Uptime) * time.Second),
		CPUPercent:  round(cpuPercent),
		RAMPercent:  round([]float64{vm.UsedPercent}),
		DiskPercent: round([]float64{usage.UsedPercent}),
	}, nil
}

func (r Report) Render() string {
	lines := []string{
		fmt.Sprintf("Host: %s", r.Hostname),
		fmt.Sprintf("Platform: %s", r.Platform),
		fmt.Sprintf("Uptime: %s", r.Uptime),
		fmt.Sprintf("CPU: %.1f%%", r.CPUPercent),
		fmt.Sprintf("RAM: %.1f%%", r.RAMPercent),
		fmt.Sprintf("Disk: %.1f%%", r.DiskPercent),
	}
	return strings.Join(lines, "\n")
}

func round(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return float64(int(values[0]*10+0.5)) / 10
}

func humanDuration(duration time.Duration) string {
	if duration < time.Minute {
		return duration.Truncate(time.Second).String()
	}
	if duration < time.Hour {
		return duration.Truncate(time.Minute).String()
	}
	return duration.Truncate(time.Minute).String()
}
