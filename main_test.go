package main

import (
	"testing"
	"net"
	"fmt"
)

func InitTest() Config {
	ep1 := Endpoints{
		Name:    "ep1",
		Weight:  3,
		Address: "10.221.2.12"}
	ep2 := Endpoints{
		Name:    "ep1",
		Weight:  1,
		Address: "10.221.2.12"}
	ep3 := Endpoints{
		Name:    "ep1",
		Weight:  6,
		Address: "10.221.2.12"}
	ra := RecordA{
		Endpoints: []Endpoints{ep1, ep2, ep3}}
	d1 := Diversion{
		Source: "10.211.99.23",
		A:      ra,
		Name:   "ip"}
	d2 := Diversion{
		Source: "10.211.0.1/16",
		Name:   "n16"}
	d3 := Diversion{
		Source: "10.211.99.1/24",
		Name:   "n24"}
	d4 := Diversion{
		Source: "10.0.0.0/8",
		Name:   "n8"}
	d5 := Diversion{
		Source: "0.0.0.0/0",
		Name:   "default"}
	v1 := VDns{
		Domain: "vu",
		Parent: "."}
	v2 := VDns{
		Domain: "vk.vu",
		Parent: "vu"}
	v3 := VDns{
		Domain:    "r.vk.vu",
		Parent:    "vk.vu",
		Diversion: []Diversion{d1, d2, d3, d4, d5}}
	v4 := VDns{
		Domain: "*.r.vk.vu",
		Parent: "r.vk.vu"}
	c1 := VDns{
		Domain: "vx",
		Parent: "."}
	v5 := VDns{
		Domain: "*.vk.vu",
		Parent: "vk.vu"}
	c = Config{[]VDns{v1, v2, v3, v4, c1, v5}, VDns{Domain: ""}}
	return c
}

func TestScanDNS(t *testing.T) {
	c := InitTest()

	c.scanDNS()

	if len(c.Base.Children) != 2 {
		t.Error("scandns not correct root should be 2")
	}

	if v := c.Base.getChild("vu").getChild("vk").Domain; v != "vk.vu" {
		t.Errorf("get child of vk.vu not correct got %s", v)
	}

	if v := c.Base.getChild("vu").getChild("vk").getChild("*").Domain; v != "*.vk.vu" {
		t.Errorf("get child of *.vk.vu not correct got %s", v)
	}
}

func TestGetDNS(t *testing.T) {
	c := InitTest()
	c.scanDNS()

	if v := c.getDNS("vk.vu.").Domain; v != "vk.vu" {
		t.Errorf("getDns vk.vu should be vk.vu got %s", v)
	}

	if v := c.getDNS("r.vk.vu").Domain; v != "r.vk.vu" {
		t.Errorf("getDns vk.vu should be vk.vu got %s", v)
	}

	if v := c.getDNS("x.vk.vu").Domain; v != "*.vk.vu" {
		t.Errorf("getDns vk.vu should be vk.vu got %s", v)
	}

	if v := c.getDNS("z.r.vk.vu").Domain; v != "*.r.vk.vu" {
		t.Errorf("getDns vk.vu should be vk.vu got %s", v)
	}

	if v := c.getDNS("z.p.vk.vu").Domain; v != "*.vk.vu" {
		t.Errorf("getDns vk.vu should be vk.vu got %s", v)
	}
}

func TestVDns_GetDiversion(t *testing.T) {
	InitTest()
	c.scanDNS()
	dns := c.getDNS("r.vk.vu")

	ip := net.ParseIP("10.211.99.23")
	if d := dns.GetDiversion(ip); d.Name != "ip" {
		t.Errorf("get Diversion %s 's name should be ip", ip)
	}

	ip = net.ParseIP("10.211.99.24")
	if d := dns.GetDiversion(ip); d.Name != "n24" {
		t.Errorf("get Diversion %s 's name should be n24 , got %s", ip, d.Name)
	}

	ip = net.ParseIP("10.211.98.24")
	if d := dns.GetDiversion(ip); d.Name != "n16" {
		t.Errorf("get Diversion %s 's name should be n16 , got %s", ip, d.Name)
	}

	ip = net.ParseIP("10.201.98.24")
	if d := dns.GetDiversion(ip); d.Name != "n8" {
		t.Errorf("get Diversion %s 's name should be n8 , got %s", ip, d.Name)
	}

	ip = net.ParseIP("9.201.98.24")
	if d := dns.GetDiversion(ip); d.Name != "default" {
		t.Errorf("get Diversion %s 's name should be default , got %s", ip, d.Name)
	}
}

func TestVDns_GetARecord(t *testing.T) {
	InitTest()
	c.scanDNS()
	dns := c.getDNS("r.vk.vu")
	ip := net.ParseIP("10.211.99.23")
	d := dns.GetDiversion(ip)
	fmt.Println(d.A)
	ep := d.A.getEndpoints()
	if ep == nil{
		t.Error("can't get endpoint")
	}
}
