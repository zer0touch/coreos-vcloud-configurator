// Copyright (c) 2015 Ryan Harper. All rights reserved.
// Use of this source code is governed by the Apache License, Version 2.0
// that can be found in the LICENSE file.
// Thanks to Kelsey Hightower for the inspiration on this, I have resorted to
// adding additional functionality to this handy little app.
package main

import (
	"bytes"
	"flag"
	"fmt"
	yaml "gopkg.in/yaml.v1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

var (
	configFile string
	iso        bool
	sshKey     string
	clustSize  int
	hostName   string
	ipAddr     string
	hostPrefix string
	coreToken  string
	platformDomain string
	channel string
)

type dest struct {
	Nodes2 map[string]map[string]interface{} `yaml:"nodes"`
}

type typedDest struct {
	Nodes2 map[string]Node2 `yaml:"nodes"`
}

type Node2 struct {
	HostName      string
	IPAddress     string
	Ext_IPAddress string
	Ext_Gateway   string
	VXLan_IP      string
	VApp          string
	Disks         map[string]string
}

type Config struct {
	DNS      string                       `yaml:"dns"`
	Gateway  string                       `yaml:"gateway"`
	MasterIP string                       `yaml:"master_ip"`
	Nodes    map[string]map[string]string `yaml:"nodes"`
	Node1IP  string                       `yaml:"node1_ip"`
	Node2IP  string                       `yaml:"node2_ip"`
	SSHKey   string                       `yaml:"sshkey"`
	ExtDNS1  string                       `yaml:"ext_dns1"`
	ExtDNS2  string                       `yaml:"ext_dns2"`
	DNS1     string                       `yaml:"dns1"`
	DNS2     string                       `yaml:"dns2"`
}

func init() {
	flag.StringVar(&configFile, "c", "kubernetes.yml", "config file to use")
	flag.StringVar(&sshKey, "k", "config.yml", "the ssh public key to use")
	flag.BoolVar(&iso, "iso", false, "generate config-drive iso images")
	flag.IntVar(&clustSize, "s", 1, "size of the cluster")
	flag.StringVar(&hostPrefix, "h", "core-", "host prefix to use")
	flag.StringVar(&coreToken, "t", "", "the coreos token to use")
	flag.StringVar(&platformDomain, "d", "skydns.local", "platform domain to use")
	flag.StringVar(&channel, "r", "stable", "generate config-drive iso images")

}

func main() {
	flag.Parse()
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err.Error())
	}
	var c Config
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	data := make(map[string]string)

	var keys []string
	for k := range c.Nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, v := range keys {
		peermap := make(map[string]map[string]string)
		for k, v := range c.Nodes {
			peermap[k] = v
		}

		delete(peermap, keys[i]) // deleting the first element of the map, so we can assign the master

		var peerarray []string
		for _, each := range peermap {
			peerarray = append(peerarray, each["ipaddress"]+":7001") //append the etcd port to each ip address in this map

		}

		peerstring := strings.Join(peerarray, ",")

		dat, err := ioutil.ReadFile(configFile)

		etcdpeermap := make(map[string]map[string]string)
		for k, v := range c.Nodes {
			etcdpeermap[k] = v
		}

        var etcdpeerarray []string
		for _, each := range etcdpeermap {
			etcdpeerarray = append(etcdpeerarray, "http://"+each["ipaddress"]+":4001") //append the etcd port to each ip address in this map

		}

		var etcd2peerarray []string
		for _, each := range etcdpeermap {
			etcd2peerarray = append(etcd2peerarray, each["hostname"]+"=http://"+each["ipaddress"]+":2380") //append the etcd port to each ip address in this map

		}

		var skydnspeerarray []string
		for _, each := range etcdpeermap {
			skydnspeerarray = append(skydnspeerarray, "forward-addr: "+each["ipaddress"]+"@9000\n") //append the etcd port to each ip address in this map

		}

		etcdpeerstring := strings.Join(etcdpeerarray, ",")
		etcd2peerstring := strings.Join(etcd2peerarray, ",")
		skydnspeerstring := strings.Join(skydnspeerarray, "         ")

		d := &dest{}
		err = yaml.Unmarshal(dat, &d)
		if err != nil {
			panic(err)
		}

		fmt.Printf("%#v\n", d)

		td := &typedDest{}
		err = yaml.Unmarshal(dat, &td)
		if err != nil {
			panic(err)
		}

		data["dns"] = c.DNS
		data["channel"] = channel
		data["sshkey"] = sshKey
		data["token"] = coreToken
		if i == 0 {
			data["role"] = "master"
		} else {
			data["role"] = "slave"
		}
		data["peers"] = peerstring
		data["etcdpeers"] = etcdpeerstring
		data["etcd2peers"] = etcd2peerstring
		data["skydnspeers"] = skydnspeerstring
		data["platformdomain"] = platformDomain
		data["hostname"] = c.Nodes[v]["hostname"]
		data["gateway"] = c.Nodes[v]["gateway"]
		data["ip"] = c.Nodes[v]["ipaddress"]
		data["vxlan_ip"] = c.Nodes[v]["vxlan_ip"]
		data["ext_ip"] = c.Nodes[v]["ext_ipaddress"]
		data["ext_gateway"] = c.Nodes[v]["ext_gateway"]
		data["ext_dns1"] = c.Nodes[v]["dns1"]
		data["ext_dns2"] = c.Nodes[v]["edns2"]
		data["dns1"] = c.Nodes[v]["dns1"]
		data["dns2"] = c.Nodes[v]["dns2"]
		data["vapp"] = c.Nodes[v]["vapp"]
		data["disk1"] = td.Nodes2[v].Disks["docker"]
		data["disk2"] = td.Nodes2[v].Disks["data"]
		data["disk3"] = td.Nodes2[v].Disks["backup"]
		render(data)
	}

}

func render(data map[string]string) {
	var buf bytes.Buffer
	f, err := os.Create(data["hostname"] + ".yml")
	if err != nil {
		log.Fatal(err.Error())
	}

	w := io.MultiWriter(f, &buf)
	if err := nodeTmpl.Execute(w, data); err != nil {
		log.Fatal(err.Error())
	}

	if iso {
		isoName := data["vapp"] + data["hostname"] + ".iso"
		resp, err := http.Post("http://127.0.0.1/genisoimage", "application/yaml", &buf)
		if err != nil {
			log.Fatal(err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Fatal("non 200 exit code")
		}
		f, err := os.Create(isoName)
		if err != nil {
			log.Fatal(err.Error())
		}
		defer f.Close()
		io.Copy(f, resp.Body)

	}
}
