//go:build windows && (amd64 || arm64)

package network

import (
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var getIPv4ForwardTable = iphlp.NewProc("GetIpForwardTable")

type ipv4ForwardRow struct {
	Destination, Mask, Policy, NextHop         uint32
	Index, Type, Protocol, Age, NextHopAS      uint32
	Metric, Metric2, Metric3, Metric4, Metric5 uint32
}

func tunnelInterface(row ifRow2) bool {
	if row.Type == 53 || row.Type == 131 {
		return true
	}
	description := strings.ToLower(syscall.UTF16ToString(row.Description[:]))
	for _, name := range []string{"tunnel", "wintun", "wireguard", "tap-windows", "openvpn"} {
		if strings.Contains(description, name) {
			return true
		}
	}
	return false
}

func trafficInterfaceUsable(row ifRow2) bool {
	return row.OperStatus == 1 && row.Type != 24 && row.Flags&2 == 0 && !tunnelInterface(row)
}

func defaultIPv4Route(row ipv4ForwardRow) bool {
	return row.Destination == 0 && row.Mask == 0 && (row.Type == 3 || row.Type == 4) && row.Metric != ^uint32(0)
}

// Keep a direct OS-selected route. For a TUN route, use one connected underlying
// default-route interface, never the sum of virtual and physical byte counters.
func selectTrafficInterface(routed ifRow2, interfaces []ifRow2, routes []ipv4ForwardRow) (ifRow2, error) {
	if trafficInterfaceUsable(routed) {
		return routed, nil
	}
	var best ifRow2
	var metric uint32
	for _, route := range routes {
		if !defaultIPv4Route(route) {
			continue
		}
		for _, row := range interfaces {
			if row.Index != route.Index || !trafficInterfaceUsable(row) {
				continue
			}
			better := best.Index == 0 || route.Metric < metric
			if route.Metric == metric && best.Index != 0 {
				hardware, bestHardware := row.Flags&1 != 0, best.Flags&1 != 0
				better = (hardware && !bestHardware) || (hardware == bestHardware && row.Index < best.Index)
			}
			if better {
				best, metric = row, route.Metric
			}
		}
	}
	if best.Index == 0 {
		return ifRow2{}, fmt.Errorf("未找到可统计流量的实际出口接口")
	}
	return best, nil
}

func ipv4Routes() ([]ipv4ForwardRow, error) {
	buffer := make([]byte, 4096)
	for attempt := 0; attempt < 3; attempt++ {
		size := uint32(len(buffer))
		code, _, _ := getIPv4ForwardTable.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0)
		if code == 122 && size > uint32(len(buffer)) && size <= 1<<20 {
			buffer = make([]byte, size)
			continue
		}
		if code != 0 {
			return nil, fmt.Errorf("读取流量出口路由: %w", syscall.Errno(code))
		}
		if size < 4 || size > uint32(len(buffer)) {
			return nil, fmt.Errorf("流量出口路由响应大小无效")
		}
		count := binary.LittleEndian.Uint32(buffer[:4])
		if uint64(count) > uint64(size-4)/uint64(unsafe.Sizeof(ipv4ForwardRow{})) {
			return nil, fmt.Errorf("流量出口路由数量无效")
		}
		rows := unsafe.Slice((*ipv4ForwardRow)(unsafe.Pointer(&buffer[4])), int(count))
		return append([]ipv4ForwardRow(nil), rows...), nil
	}
	return nil, fmt.Errorf("流量出口路由在采样期间发生变化")
}

func trafficInterface(routed ifRow2) (ifRow2, error) {
	if trafficInterfaceUsable(routed) {
		return routed, nil
	}
	routes, err := ipv4Routes()
	if err != nil {
		return ifRow2{}, err
	}
	var rows []ifRow2
	seen := make(map[uint32]bool)
	for _, route := range routes {
		if !defaultIPv4Route(route) || seen[route.Index] {
			continue
		}
		seen[route.Index] = true
		row, err := readInterface(route.Index)
		if err == nil {
			rows = append(rows, row)
		}
	}
	return selectTrafficInterface(routed, rows, routes)
}
