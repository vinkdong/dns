package main

import (
	"github.com/miekg/dns"
	"strings"
	"net"
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"fmt"
	"github.com/vinkdong/gox/log"
	"os"
)

var (
	ttl    = flag.Uint("ttl", 600, "time to live")
	config = flag.String("conf", "", "server config file")
	proxy  = flag.String("proxy,", "8.8.8.8", "default proxy dns server")
	debug  = flag.Bool("debug", false, "debug model")
	base   = VDns{Domain: ""}
)

type Config struct {
	Dns  []VDns
	Base VDns
}

func (c *Config) search(domain string) *VDns {
	for _, v := range c.Dns {
		if v.Domain == domain {
			return &v
		}
	}
	return nil
}

func (c *Config) getParent(d *VDns) *VDns {
	for k, v := range c.Dns {
		if &v != d && v.Domain == d.Parent {
			return &c.Dns[k]
		}
	}
	return nil
}

func (d *VDns) getChild(key string) *VDns {
	domain := key + "." + d.Domain
	for _, v := range d.Children {
		if v.Domain == domain || v.Domain+"." == domain {
			return v
		}
	}
	return nil
}

type VDns struct {
	Domain    string
	Parent    string
	Key       string
	Diversion []Diversion `yaml:"diversion"`
	Children  []*VDns
}

type Diversion struct {
	A      RecordA `yaml:"a"`
	Source string
	Name   string
	Net    *net.IPNet
	IP     net.IP
}

func (d *VDns) ScanDiversion() {
	for k, v := range d.Diversion {
		ip := net.ParseIP(v.Source)
		diversion := &d.Diversion[k]
		if ip == nil {
			_, net, err := net.ParseCIDR(v.Source)
			if err != nil {
				log.Error(err)
				os.Exit(127)
			}
			diversion.Net = net
		}
		diversion.IP = ip
	}
}

func (d *VDns) GetDiversion(ip net.IP) *Diversion {
	size := 0
	var crDiversion *Diversion
	for k, v := range d.Diversion {
		if v.IP.Equal(ip) {
			return &d.Diversion[k]
		}
		if v.Net != nil && v.Net.Contains(ip) {
			ones, _ := v.Net.Mask.Size()
			if ones >= size {
				crDiversion = &d.Diversion[k]
			}
			size = ones
		}
	}
	return crDiversion
}

type RecordA struct {
	Endpoints []Endpoints
	net       net.IPNet
	ip        net.IP
}

// 加权轮询
func (a *RecordA) getEndpoints() *Endpoints {
	var min *Endpoints
	var minCalVal = float32(-1)
	for k, v := range a.Endpoints {
		calVal := float32(v.execTimes) / float32(v.Weight)
		if minCalVal < 0 || calVal <= minCalVal {
			min = &a.Endpoints[k]
			minCalVal = calVal
		}
	}
	min.execTimes += 1
	return min
}

type Endpoints struct {
	Name      string
	Weight    int
	Address   string
	execTimes int
}

var c Config

func main() {

	flag.Parse()
	b, err := ioutil.ReadFile(*config)
	if err != nil {
		fmt.Errorf("%v", err)
	}

	c = Config{}
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		fmt.Errorf("%v", err)
	}

	c.scanDNS()

	dns.HandleFunc(".", ServeDNS)

	go func() {
		srv := &dns.Server{Addr: ":53", Net: "udp", Handler: nil}
		err := srv.ListenAndServe()
		if err != nil {
			log.Errorf("Error for set udp listener %s\n", err)
		}
	}()
	log.Info("run dns server on port 53")
	srv := &dns.Server{Addr: ":53", Net: "tcp", Handler: nil}
	err = srv.ListenAndServe()
	if err != nil {
		log.Errorf("Error for set tcp listener %s\n", err)
	}
}

func (c *Config) scanDNS() {
	base := &c.Base

	for k, v := range c.Dns {
		p := &c.Dns[k]
		if v.Parent == "." {
			base.Children = append(base.Children, p)
		} else {
			parent := c.getParent(&v)
			if parent != nil {
				parent.Children = append(parent.Children, p)
			} else {
				log.Errorf("can't find %s's parent", v.Domain)
			}
		}
		// init diversion
		p.ScanDiversion()
	}
}

func (c *Config) getDNS(domain string) *VDns {
	sp := strings.Split(domain, ".")
	crDns := &c.Base
	for i := len(sp) - 1; i >= 0; i -- {
		if sp[i] == "" {
			continue
		}
		nextDns := crDns.getChild(sp[i])
		if nextDns == nil {
			nextDns = crDns.getChild("*")
			if nextDns == nil {
				log.Errorf("cant get domain %s", domain)
			}
			return nextDns
		}
		crDns = nextDns
	}
	return crDns
}

func ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	domain := strings.ToLower(r.Question[0].Name)

	ip, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	log.Infof("receive request from %s\t%s\n", ip, domain)

	var msg *dns.Msg

	msg = new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true


	vdns := c.getDNS(domain)
	if vdns == nil {
		msg.SetRcode(r, 2)
		w.WriteMsg(msg)
		return
	}

	d := vdns.GetDiversion(net.ParseIP(ip))
	if d == nil {
		msg.SetRcode(r, 2)
		w.WriteMsg(msg)
		return
	}
	ep := d.A.getEndpoints()
	if ep == nil {
		msg.SetRcode(r, 2)
		w.WriteMsg(msg)
		return
	}

	if true {
		msg = new(dns.Msg)
		msg.SetReply(r)
		msg.Authoritative = true
		rrA := new(dns.A)
		rrA.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(*ttl)}
		rrA.A = net.ParseIP(ep.Address)
		msg.Answer = []dns.RR{rrA}
	} else {
		msg.SetRcode(r, 2)
		w.WriteMsg(msg)
	}
	w.WriteMsg(msg)
}
