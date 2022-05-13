package cpu

import (
	"path"
	"strconv"
	"strings"
)

var cgroupSet map[string]string

func init() {
	cgroupSet = make(map[string]string)
	cgroupSet["freezer"] = "/sys/fs/cgroup/freezer"
	cgroupSet["rdma"] = "/sys/fs/cgroup/rdma"
	cgroupSet["net_cls"] = "/sys/fs/cgroup/net_cls"
	cgroupSet["memory"] = "/sys/fs/cgroup/memory"
	cgroupSet["pids"] = "/sys/fs/cgroup/pids"
	cgroupSet["net_prio"] = "/sys/fs/cgroup/net_prio"
	cgroupSet["devices"] = "/sys/fs/cgroup/devices"
	cgroupSet["perf_event"] = "/sys/fs/cgroup/perf_event"
	cgroupSet[""] = "/sys/fs/cgroup"
	cgroupSet["cpuset"] = "/sys/fs/cgroup/cpuset"
	cgroupSet["hugetlb"] = "/sys/fs/cgroup/hugetlb"
	cgroupSet["net_cls,net_prio"] = "/sys/fs/cgroup/net_cls,net_prio"
	cgroupSet["blkio"] = "/sys/fs/cgroup/blkio"
	cgroupSet["cpu,cpuacct"] = "/sys/fs/cgroup/cpu,cpuacct"
	cgroupSet["cpu"] = "/sys/fs/cgroup/cpu"
	cgroupSet["cpuacct"] = "/sys/fs/cgroup/cpuacct"
	cgroupSet["name=systemd"] = "/sys/fs/cgroup/name=systemd"
}

// cgroup Linux cgroup
type cgroup struct {
	cgroupSet map[string]string
}

// CPUCFSQuotaUs cpu.cfs_quota_us
func (c *cgroup) CPUCFSQuotaUs() (int64, error) {
	// data, err := readFile(path.Join(c.cgroupSet["cpuset"], "cpu.cfs_quota_us"))
	data, err := remoteGetSourceString(path.Join(c.cgroupSet["cpu"], "cpu.cfs_quota_us"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(data, 10, 64)
}

// CPUCFSPeriodUs cpu.cfs_period_us
func (c *cgroup) CPUCFSPeriodUs() (uint64, error) {
	// data, err := readFile(path.Join(c.cgroupSet["cpu"], "cpu.cfs_period_us"))
	data, err := remoteGetSourceString(path.Join(c.cgroupSet["cpu"], "cpu.cfs_period_us"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsage cpuacct.usage
func (c *cgroup) CPUAcctUsage() (uint64, error) {
	// data, err := readFile(path.Join(c.cgroupSet["cpuacct"], "cpuacct.usage"))
	data, err := remoteGetSourceString(path.Join(c.cgroupSet["cpuacct"], "cpuacct.usage"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsagePerCPU cpuacct.usage_percpu
func (c *cgroup) CPUAcctUsagePerCPU() ([]uint64, error) {
	// data, err := readFile(path.Join(c.cgroupSet["cpuacct"], "cpuacct.usage_percpu"))
	data, err := remoteGetSourceString(path.Join(c.cgroupSet["cpuacct"], "cpuacct.usage_percpu"))
	if err != nil {
		return nil, err
	}
	var usage []uint64
	for _, v := range strings.Fields(data) {
		var u uint64
		if u, err = parseUint(v); err != nil {
			return nil, err
		}
		// fix possible_cpu:https://www.ibm.com/support/knowledgecenter/en/linuxonibm/com.ibm.linux.z.lgdd/lgdd_r_posscpusparm.html
		if u != 0 {
			usage = append(usage, u)
		}
	}
	return usage, nil
}

// CPUSetCPUs cpuset.cpus
func (c *cgroup) CPUSetCPUs() ([]uint64, error) {
	// data, err := readFile(path.Join(c.cgroupSet["cpuset"], "cpuset.cpus"))
	data, err := remoteGetSourceString(path.Join(c.cgroupSet["cpuset"], "cpuset.cpus"))
	if err != nil {
		return nil, err
	}
	cpus, err := ParseUintList(data)
	if err != nil {
		return nil, err
	}
	sets := make([]uint64, len(cpus))
	i := 0
	for k := range cpus {
		sets[i] = uint64(k)
		i++
	}

	return sets, nil
}

// CurrentcGroup get current process cgroup
func currentcGroup() (*cgroup, error) {
	return &cgroup{cgroupSet: cgroupSet}, nil
}
