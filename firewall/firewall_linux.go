package firewall

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/evilsocket/bettercap-ng/core"
)

type LinuxFirewall struct {
	forwarding   bool
	redirections map[string]*Redirection
}

const (
	IPV4ForwardingFile    = "/proc/sys/net/ipv4/ip_forward"
	IPV4ICMPBcastFile     = "/proc/sys/net/ipv4/icmp_echo_ignore_broadcasts"
	IPV4SendRedirectsFile = "/proc/sys/net/ipv4/conf/all/send_redirects"
)

func Make() FirewallManager {
	firewall := &LinuxFirewall{
		forwarding:   false,
		redirections: make(map[string]*Redirection, 0),
	}

	firewall.forwarding = firewall.IsForwardingEnabled()

	return firewall
}

func (f LinuxFirewall) enableFeature(filename string, enable bool) error {
	var value string
	if enable {
		value = "1"
	} else {
		value = "0"
	}

	fd, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer fd.Close()

	if _, err = fd.WriteString(value); err != nil {
		return err
	}

	return nil
}

func (f LinuxFirewall) IsForwardingEnabled() bool {

	if out, err := ioutil.ReadFile(IPV4ForwardingFile); err != nil {
		return false
	} else {
		return strings.Trim(string(out), "\r\n\t ") == "1"
	}
}

func (f LinuxFirewall) EnableForwarding(enabled bool) error {
	return f.enableFeature(IPV4ForwardingFile, enabled)
}

func (f LinuxFirewall) EnableIcmpBcast(enabled bool) error {
	return f.enableFeature(IPV4ICMPBcastFile, enabled)
}

func (f LinuxFirewall) EnableSendRedirects(enabled bool) error {
	return f.enableFeature(IPV4SendRedirectsFile, enabled)
}

func (f *LinuxFirewall) EnableRedirection(r *Redirection, enabled bool) error {
	var opts []string

	rkey := r.String()
	_, found := f.redirections[rkey]

	if enabled == true {
		if found == true {
			return fmt.Errorf("Redirection '%s' already enabled.", rkey)
		}

		f.redirections[rkey] = r

		// accept all
		if _, err := core.Exec("iptables", []string{"-P", "FORWARD", "ACCEPT"}); err != nil {
			return err
		}

		if r.SrcAddress == "" {
			opts = []string{
				"-t", "nat",
				"-A", "PREROUTING",
				"-i", r.Interface,
				"-p", r.Protocol,
				"--dport", fmt.Sprintf("%d", r.SrcPort),
				"-j", "DNAT",
				"--to", fmt.Sprintf("%s:%d", r.DstAddress, r.DstPort),
			}
		} else {
			opts = []string{
				"-t", "nat",
				"-A", "PREROUTING",
				"-i", r.Interface,
				"-p", r.Protocol,
				"-d", r.SrcAddress,
				"--dport", fmt.Sprintf("%d", r.SrcPort),
				"-j", "DNAT",
				"--to", fmt.Sprintf("%s:%d", r.DstAddress, r.DstPort),
			}
		}
		if _, err := core.Exec("iptables", opts); err != nil {
			return err
		}
	} else {
		if found == false {
			return nil
		}

		delete(f.redirections, r.String())

		if r.SrcAddress == "" {
			opts = []string{
				"-t", "nat",
				"-D", "PREROUTING",
				"-i", r.Interface,
				"-p", r.Protocol,
				"--dport", fmt.Sprintf("%d", r.SrcPort),
				"-j", "DNAT",
				"--to", fmt.Sprintf("%s:%d", r.DstAddress, r.DstPort),
			}
		} else {
			opts = []string{
				"-t", "nat",
				"-D", "PREROUTING",
				"-i", r.Interface,
				"-p", r.Protocol,
				"-d", r.SrcAddress,
				"--dport", fmt.Sprintf("%d", r.SrcPort),
				"-j", "DNAT",
				"--to", fmt.Sprintf("%s:%d", r.DstAddress, r.DstPort),
			}
		}
		if _, err := core.Exec("iptables", opts); err != nil {
			return err
		}
	}

	return nil
}

func (f LinuxFirewall) Restore() {
	for _, r := range f.redirections {
		if err := f.EnableRedirection(r, false); err != nil {
			fmt.Printf("%s", err)
		}
	}

	if err := f.EnableForwarding(f.forwarding); err != nil {
		fmt.Printf("%s", err)
	}
}
