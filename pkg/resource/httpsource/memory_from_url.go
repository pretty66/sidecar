package httpsource

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/openmsp/sidecar/pkg/resource"
)

// Doc: https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
// Reference: http://linuxperf.com/?p=142

func (hs *HTTPSource) CurrentMemStat() (stat *resource.MemStat, err error) {
	var m map[string]uint64
	m, err = resource.ReadMapFromRemote(hs.remoteURL, "/sys/fs/cgroup/memory/memory.stat")
	if err != nil {
		return nil, err
	}

	stat = &resource.MemStat{}
	stat.Total, err = hs.totalMemory(m)
	if err != nil {
		return nil, err
	}

	stat.SwapTotal, stat.SwapUsed = hs.swapState(m)

	stat.Cached = m["total_cache"]
	stat.MappedFile = m["total_mapped_file"]
	memoryUsageInBytes, err := resource.ReadIntFromRemote(hs.remoteURL, "/sys/fs/cgroup/memory/memory.usage_in_bytes")
	if err != nil {
		stat.RSS = m["total_rss"] + stat.MappedFile
	} else {
		if v, ok := m["total_inactive_file"]; ok {
			if uint64(memoryUsageInBytes) < v {
				memoryUsageInBytes = 0
			} else {
				memoryUsageInBytes -= int64(v)
			}
		}
		stat.RSS = uint64(memoryUsageInBytes)
	}
	return stat, err
}

func (hs *HTTPSource) getHostMemTotal() (n uint64, err error) {
	var scanner *bufio.Scanner

	ret, err := resource.RemoteGetSourceByte(hs.remoteURL, "/proc/meminfo")
	if err != nil {
		return 0, err
	}
	reader := bytes.NewReader(ret)
	scanner = bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] != "MemTotal:" {
			continue
		}
		parts[1] = strings.TrimSpace(parts[1])
		value := strings.TrimSuffix(parts[1], "kB")
		value = strings.TrimSpace(value)
		n, err = strconv.ParseUint(value, 10, 64)
		n *= 1024
		if err != nil {
			return 0, err
		}
		break
	}
	return
}

func (hs *HTTPSource) totalMemory(m map[string]uint64) (uint64, error) {
	hostTotal, err := hs.getHostMemTotal()
	if err != nil {
		return 0, err
	}
	limit, ok := m["hierarchical_memory_limit"]
	if !ok {
		return 0, fmt.Errorf("missing hierarchical_memory_limit")
	}
	if hostTotal > limit {
		return limit, nil
	}
	return hostTotal, nil
}

func (hs *HTTPSource) swapState(m map[string]uint64) (total uint64, used uint64) {
	memSwap, ok := m["hierarchical_memsw_limit"]
	if !ok {
		return 0, 0
	}

	mem := m["hierarchical_memory_limit"]
	if memSwap == mem {
		return 0, 0
	}

	total = memSwap - mem
	used = m["total_swap"]
	return total, used
}

func (hs *HTTPSource) GetRss() (int64, error) {
	return 0, nil
}
