[![Build Status](https://travis-ci.org/zer0touch/coreos-vcloud-configurator.svg?branch=master)](https://travis-ci.org/zer0touch/coreos-vcloud-configurator)
# CoreOS vcloud-configurator

This is a small utility for generating cloud-init files for coreos and packages them up into an iso to be used with vcloud director.  At the time of writing vcloud director did not support coreos directly and the vmware-tools (opensource/prop) were not packaged to run on coreos.  As a result some of the feedback mechanisms that vmware-tools offered other operating systems (e.g interogating the ip address, default gateway and networks) were not available, thereby making it extremely difficult to automate provisioning end to end.  This tools aims to work around some of these shortfalls by precreating the config disks, uploading them to vcloud and then attaching them to the instance.  A really good side effect of this method is that the configuration is immutable so any changes do that are not reflected in an updated config drive will effectively get wiped.  This configuration was specifically used to create HA coreos clusters to run various services and tasks on. 

The code is heavily borrowed from Kelsey Hightower and a big shout out to him for making this available initially.  

## Usage

```
Usage of ./coreos-vcloud-configurator
:
  -c="kubernetes.yml": config file to use
  -iso=false: generate config-drive iso images
```

config.yml
```
dns: 192.168.12.1
gateway: 192.168.12.1
master_ip: 192.168.12.10
node1_ip: 192.168.12.11
node2_ip: 192.168.12.12
sshkey: ssh-rsa AAAAB3NzaC1yc2...
```

### Create cloud-config files

```
coreos-vcloud-configurator  -c config.yml
```
-

```
master.yml node1.yml node2.yml
```

### Create config-drive iso images

The following command will connect to a remote ISO creation service.

```
coreos-vcloud-configurator -c config.yml -iso
```

-

```
The resulting output should appear with the following files and configurations. 
master.iso  master.yml  node1.iso   node1.yml   node2.iso   node2.yml
```
