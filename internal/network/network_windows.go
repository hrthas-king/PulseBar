//go:build windows && (amd64 || arm64)

// Package network 使用 Windows IP Helper API 读取出口网卡 64 位计数、发送 ICMP 探针并监听路由事件。
package network

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/hrthas-king/PulseBar/internal/monitor"
	"github.com/hrthas-king/PulseBar/internal/winapi"
)

var (
	iphlp            = winapi.DLL("iphlpapi.dll")
	ws2_32           = winapi.DLL("ws2_32.dll")
	getBestInterface = iphlp.NewProc("GetBestInterface")
	getIfEntry2      = iphlp.NewProc("GetIfEntry2")
	icmpCreate       = iphlp.NewProc("IcmpCreateFile")
	icmpSend         = iphlp.NewProc("IcmpSendEcho")
	icmpClose        = iphlp.NewProc("IcmpCloseHandle")
	notifyInterface  = iphlp.NewProc("NotifyIpInterfaceChange")
	notifyAddress    = iphlp.NewProc("NotifyUnicastIpAddressChange")
	notifyRoute      = iphlp.NewProc("NotifyRouteChange2")
	cancelNotify     = iphlp.NewProc("CancelMibChangeNotify2")
	wsaStartup       = ws2_32.NewProc("WSAStartup")
	wsaSocket        = ws2_32.NewProc("socket")
	wsaIoctlSocket   = ws2_32.NewProc("ioctlsocket")
	wsaConnect       = ws2_32.NewProc("connect")
	wsaSelect        = ws2_32.NewProc("select")
	wsaCloseSocket   = ws2_32.NewProc("closesocket")
)

type ifRow2 struct {
	LUID                                                                            uint64
	Index                                                                           uint32
	GUID                                                                            [16]byte
	Alias                                                                           [257]uint16
	Description                                                                     [257]uint16
	PhysicalAddressLength                                                           uint32
	PhysicalAddress                                                                 [32]byte
	PermanentPhysicalAddress                                                        [32]byte
	MTU, Type, TunnelType, MediaType, PhysicalMediumType, AccessType, DirectionType uint32
	Flags                                                                           uint8
	_                                                                               [3]byte
	OperStatus, AdminStatus, MediaConnectState                                      uint32
	NetworkGUID                                                                     [16]byte
	ConnectionType                                                                  uint32
	_                                                                               [4]byte
	TransmitLinkSpeed, ReceiveLinkSpeed                                             uint64
	InOctets, InUcastPkts, InNUcastPkts, InDiscards, InErrors, InUnknownProtos      uint64
	InUcastOctets, InMulticastOctets, InBroadcastOctets                             uint64
	OutOctets, OutUcastPkts, OutNUcastPkts, OutDiscards, OutErrors                  uint64
	OutUcastOctets, OutMulticastOctets, OutBroadcastOctets, OutQLen                 uint64
}

func ipv4(target string) (uint32, error) {
	addr, err := netip.ParseAddr(target)
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("无效 IPv4 探针目标 %q", target)
	}
	a := addr.As4()
	return binary.LittleEndian.Uint32(a[:]), nil
}

// ReadCounters 只读取当前路由出口接口的 64 位总字节计数，避免 VPN 与物理网卡重复累加。
func ReadCounters(target string) (monitor.Counters, error) {
	ip, err := ipv4(target)
	if err != nil {
		return monitor.Counters{}, err
	}
	var index uint32
	code, _, _ := getBestInterface.Call(uintptr(ip), uintptr(unsafe.Pointer(&index)))
	if code != 0 {
		return monitor.Counters{}, fmt.Errorf("找不到 IPv4 出口路由: %w", syscall.Errno(code))
	}
	row := ifRow2{Index: index}
	code, _, _ = getIfEntry2.Call(uintptr(unsafe.Pointer(&row)))
	if code != 0 {
		return monitor.Counters{}, fmt.Errorf("读取网卡计数: %w", syscall.Errno(code))
	}
	if row.OperStatus != 1 {
		return monitor.Counters{}, fmt.Errorf("出口网卡未连接")
	}
	return monitor.Counters{
		LUID:     row.LUID,
		Index:    row.Index,
		Name:     syscall.UTF16ToString(row.Alias[:]),
		Sent:     row.OutOctets,
		Received: row.InOctets,
		At:       time.Now(),
	}, nil
}

type Pinger struct {
	handle  uintptr
	payload [32]byte
	reply   [256]uint64
}

func NewPinger() (*Pinger, error) {
	h, _, err := icmpCreate.Call()
	if h == ^uintptr(0) || h == 0 {
		return nil, winapi.Error("IcmpCreateFile", err)
	}
	p := &Pinger{handle: h}
	copy(p.payload[:], "PulseBar connectivity heartbeat!")
	return p, nil
}

func (p *Pinger) Close() {
	if p.handle != 0 {
		icmpClose.Call(p.handle)
		p.handle = 0
	}
}

func (p *Pinger) Ping(target string) monitor.Probe {
	defer func() { runtime.KeepAlive(p) }()
	if _, err := ReadCounters(target); err != nil {
		return monitor.Probe{
			Target: target,
			Uplink: monitor.UplinkLocalError,
			Detail: err.Error(),
			At:     time.Now(),
		}
	}

	// 1. 优先尝试主目标 ICMP Echo
	res := p.pingICMP(target, 550)
	if res.Success {
		return res
	}

	// 2. 同周期立即对备用目标发起二次 ICMP 复核，避免 TUN 偶发丢弃单包导致跨秒状态闪烁
	alt := "8.8.8.8"
	if target == "8.8.8.8" {
		alt = "1.1.1.1"
	}
	resAlt := p.pingICMP(alt, 550)
	if resAlt.Success {
		return resAlt
	}

	// 3. 若 TUN/代理节点禁 Ping 或丢弃三层 ICMP，发起轻量 TCP 443 握手兜底
	if tcpRes, ok := probeTCP443(target, alt, 500*time.Millisecond); ok {
		return tcpRes
	}

	return res
}

func (p *Pinger) pingICMP(target string, timeoutMs uintptr) monitor.Probe {
	result := monitor.Probe{Target: target, Method: "icmp", Uplink: monitor.UplinkUnknown}
	ip, err := ipv4(target)
	if err != nil {
		result.Uplink = monitor.UplinkLocalError
		result.Detail = err.Error()
		result.At = time.Now()
		return result
	}
	n, _, callErr := icmpSend.Call(
		p.handle,
		uintptr(ip),
		uintptr(unsafe.Pointer(&p.payload[0])),
		32,
		0,
		uintptr(unsafe.Pointer(&p.reply[0])),
		uintptr(unsafe.Sizeof(p.reply)),
		timeoutMs,
	)
	result.At = time.Now()
	if n == 0 {
		if callErr == syscall.Errno(5) || callErr == syscall.Errno(10013) || callErr == syscall.Errno(11050) {
			result.Uplink = monitor.UplinkLocalError
		}
		result.Detail = fmt.Sprintf("ICMP 未响应/失败: %v", callErr)
		return result
	}
	words := (*[4]uint32)(unsafe.Pointer(&p.reply[0]))
	status := words[1]
	dataSize := uint16(words[3])
	if status != 0 || dataSize < 32 {
		result.Detail = fmt.Sprintf("ICMP 状态 %d，响应数据 %d 字节", status, dataSize)
		return result
	}
	result.Success = true
	result.Uplink = monitor.UplinkConfirmed
	rttMs := words[2]
	result.RTT = time.Duration(rttMs) * time.Millisecond
	return result
}

func probeTCP443(primary, secondary string, timeout time.Duration) (monitor.Probe, bool) {
	var wsaData [408]byte
	wsaStartup.Call(0x0202, uintptr(unsafe.Pointer(&wsaData[0])))
	for _, host := range []string{primary, secondary} {
		ip, err := ipv4(host)
		if err != nil {
			continue
		}
		if rtt, ok := dialTCP443(ip, timeout); ok {
			return monitor.Probe{
				Success: true,
				Method:  "tcp",
				Uplink:  monitor.UplinkConfirmed,
				RTT:     rtt,
				Target:  host,
				At:      time.Now(),
			}, true
		}
	}
	return monitor.Probe{}, false
}

func dialTCP443(ipv4LE uint32, timeout time.Duration) (time.Duration, bool) {
	s, _, _ := wsaSocket.Call(2, 1, 6) // AF_INET, SOCK_STREAM, IPPROTO_TCP
	if s == ^uintptr(0) || s == 0 {
		return 0, false
	}
	defer wsaCloseSocket.Call(s)

	nonBlock := uint32(1)
	wsaIoctlSocket.Call(s, 0x8004667E, uintptr(unsafe.Pointer(&nonBlock))) // FIONBIO

	var sa struct {
		Family uint16
		Port   uint16
		Addr   uint32
		Zero   [8]byte
	}
	sa.Family = 2
	sa.Port = 0xBB01 // htons(443)
	sa.Addr = ipv4LE

	start := time.Now()
	rc, _, _ := wsaConnect.Call(s, uintptr(unsafe.Pointer(&sa)), 16)
	if int32(rc) == 0 {
		rtt := time.Since(start)
		return rtt, true
	}

	var writeSet struct {
		Count uint32
		_     uint32
		Array [1]uintptr
	}
	writeSet.Count = 1
	writeSet.Array[0] = s

	var tv struct {
		Sec  int32
		Usec int32
	}
	us := timeout.Microseconds()
	tv.Sec = int32(us / 1_000_000)
	tv.Usec = int32(us % 1_000_000)

	n, _, _ := wsaSelect.Call(0, 0, uintptr(unsafe.Pointer(&writeSet)), 0, uintptr(unsafe.Pointer(&tv)))
	if int32(n) > 0 {
		rtt := time.Since(start)
		return rtt, true
	}
	return 0, false
}

type Watcher struct {
	Events   chan struct{}
	handles  []uintptr
	callback uintptr
}

func Watch() (*Watcher, error) {
	w := &Watcher{Events: make(chan struct{}, 1)}
	w.callback = syscall.NewCallback(func(context, row, kind uintptr) uintptr {
		select {
		case w.Events <- struct{}{}:
		default:
		}
		return 0
	})
	for _, proc := range []*syscall.LazyProc{notifyInterface, notifyAddress, notifyRoute} {
		var handle uintptr
		code, _, _ := proc.Call(0, w.callback, 0, 0, uintptr(unsafe.Pointer(&handle)))
		if code != 0 {
			w.Close()
			return w, fmt.Errorf("%s: %w", proc.Name, syscall.Errno(code))
		}
		w.handles = append(w.handles, handle)
	}
	return w, nil
}

func (w *Watcher) Close() {
	for _, h := range w.handles {
		cancelNotify.Call(h)
	}
	w.handles = nil
	runtime.KeepAlive(w)
}
