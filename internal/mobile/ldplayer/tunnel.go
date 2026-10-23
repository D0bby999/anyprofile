// SPDX-License-Identifier: AGPL-3.0-or-later

package ldplayer

import (
	"fmt"
	"strconv"
	"strings"
)

// The proxy reaches Android through SocksTun (github.com/heiher/sockstun, MIT,
// on hev-socks5-tunnel): a VpnService that sends every connection of every
// app to a SOCKS5 server, here AnyProfile's relay on the host, reached over
// `adb reverse`. With remote DNS, names go through the proxy too (no UDP
// needed); UDP itself would need UDP ASSOCIATE, which the relay refuses.
// Android's always-on VPN with lockdown blocks every connection the tunnel
// does not carry, so a stopped tunnel is no network, not the host's.
const (
	tunnelPkg     = "hev.sockstun"
	tunnelService = tunnelPkg + "/.TProxyService"
	tunnelConnect = "hev.sockstun.CONNECT"
	tunnelVersion = "8.0"
	tunnelURL     = "https://github.com/heiher/sockstun/releases/download/8.0/hev.sockstun-8.0-release.apk"
	tunnelSHA256  = "c0d76c042974e9ca1333d135562d759604b62a4ae2adedc177aaf1f38a8e4d25"
	tunnelPrefs   = "/data/data/" + tunnelPkg + "/shared_prefs/SocksPrefs.xml"
)

// tunnelPrefsXML is SocksTun's settings (Preferences.java, profile 0):
// AnyProfile's relay at 127.0.0.1:port, every app, IPv4 and IPv6 into the
// tunnel (IPv6 has no other way out then), remote DNS.
func tunnelPrefsXML(port int) string {
	b := &strings.Builder{}
	b.WriteString("<?xml version='1.0' encoding='utf-8' standalone='yes' ?>\n<map>\n")
	ints := [][2]string{{"ProfileCount", "1"}, {"Selected", "0"}, {"P0.SocksPort", strconv.Itoa(port)}}
	strs := [][2]string{{"P0.Name", "AnyProfile"}, {"P0.SocksAddr", "127.0.0.1"}, {"P0.SocksUdpAddr", ""},
		{"P0.SocksUser", ""}, {"P0.SocksPass", ""}, {"P0.DnsIpv4", "1.1.1.1"}, {"P0.DnsIpv6", "2606:4700:4700::1111"}}
	bools := [][2]string{{"Enable", "true"}, {"P0.Ipv4", "true"}, {"P0.Ipv6", "true"}, {"P0.Global", "true"},
		{"P0.UdpInTcp", "false"}, {"P0.RemoteDNS", "true"}}
	for _, kv := range ints {
		fmt.Fprintf(b, "    <int name=%q value=%q />\n", kv[0], kv[1])
	}
	for _, kv := range strs {
		fmt.Fprintf(b, "    <string name=%q>%s</string>\n", kv[0], kv[1])
	}
	for _, kv := range bools {
		fmt.Fprintf(b, "    <boolean name=%q value=%q />\n", kv[0], kv[1])
	}
	b.WriteString("</map>\n")
	return b.String()
}

// tunnelSetup is the root shell line that installs the settings pushed to
// tmp and makes the tunnel the always-on VPN with lockdown. The app is
// stopped first so it cannot write its old settings back.
func tunnelSetup(tmp string) string {
	dir := "/data/data/" + tunnelPkg + "/shared_prefs"
	return strings.Join([]string{
		"am force-stop " + tunnelPkg,
		"mkdir -p " + dir,
		"cp " + tmp + " " + tunnelPrefs,
		"rm " + tmp,
		"u=$(stat -c %u /data/data/" + tunnelPkg + ")",
		"chown $u:$u " + dir + " " + tunnelPrefs,
		"chmod 771 " + dir,
		"chmod 660 " + tunnelPrefs,
		"restorecon -R " + dir,
		"appops set " + tunnelPkg + " ACTIVATE_VPN allow",
		"settings put secure always_on_vpn_app " + tunnelPkg,
		"settings put secure always_on_vpn_lockdown 1",
		"am startservice -n " + tunnelService + " -a " + tunnelConnect,
	}, " && ")
}

// tunnelOff is for a device the user gave no proxy: no VPN, the network as is.
func tunnelOff() string {
	return strings.Join([]string{
		"settings put secure always_on_vpn_lockdown 0",
		"settings delete secure always_on_vpn_app",
		"am force-stop " + tunnelPkg,
	}, "; ")
}
