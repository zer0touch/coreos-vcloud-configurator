package main

import (
	"text/template"
)

var nodeTmpl = template.Must(template.New("node").Parse(`#cloud-config

hostname: {{.hostname}}

write_files:
  - path: /etc/systemd/system/user-cloudconfigfile.service
    content: |
      [Unit]
      Description=Load cloud-config from /media/configdrive
      Requires=coreos-setup-environment.service
      After=coreos-setup-environment.service system-config.target
      Before=user-config.target
      ConditionPathExists=/root/configdrive

      [Service]
      Type=oneshot
      RemainAfterExit=yes
      EnvironmentFile=-/etc/environment
      ExecStart=/usr/bin/coreos-cloudinit --from-file=/root/configdrive/openstack/latest/user_data
      RemainAfterExit=true
      [Install]
      WantedBy=multi-user.target
      RequiredBy=flanneld.service
      RequiredBy=early-docker.service
      RequiredBy=docker.service
      RequiredBy=etcd2.service
      RequiredBy=fleet.service
    permissions: 0640
  - path: /etc/systemd/system/docker.service.d/limits.conf
    content: |
      [Service]
      LimitNOFILE=4096
    permissions: 0640
  - path: /var/run/unbound.conf
    content: |
      server:
      verbosity: 1
      num-threads: 1
      do-ip6: no
      interface: 0.0.0.0
      interface-automatic: yes
      access-control: 127.0.0.0/8 allow
      access-control: 0.0.0.0/0 allow
      prefetch: yes
      cache-max-ttl: 1

      forward-zone:
         name: "skydns.local."
         {{.skydnspeers}}

      forward-zone:
         name: "."
         forward-addr: {{.dns1}}
         forward-addr: {{.dns2}}
    permissions: 0640
  - path: /etc/environment
    content: |
      COREOS_PUBLIC_IPV4={{.ip}}
      MON_IP={{.ip}}
      ETCDCTL_PEERS={{.etcdpeers}}
      SKYDNS_NAMESERVERS={{.dns1}}:53,{{.dns2}}:53
      ETCD_MACHINES={{.etcdpeers}}
      SKYDNS_ADDR={{.ip}}:9000
      SKYDNS_PEERS={{.etcdpeers}}
      PATH=/opt/bin:/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:/root/bin
      PLATFORM_DOMAIN={{.platformdomain}}
      ETCD_INITIAL_CLUSTER={{.etcd2peers}}
    permissions: 0644
  - path: /etc/consul.d/discovery.json
    content: |
      {
        "service": {
          "name": "discovery",
          "tags": ["discovery"],
          "port": 7001,
          "check": {
            "script": "/usr/bin/systemctl status etcd2.service",
            "interval": "60s"
          }
        }
      }
    permissions: 0640
    owner: consul
  - path: /opt/bin/diskinit.sh
    content: |
      mkdir /data ;
      mkdir /backups ;
      chown -PR root:docker /data /backups ;
      chmod 770 /data /backups
    permissions: 0550
    owner: root
users:
  - name: core
    passwd: $1$845YBKfF$KRXc5AMVJ0CtgpGVr7.nB/
    ssh-authorized-keys:
      - {{.sshkey}}
    groups:
      - sudo
      - docker
  - name: root
    passwd: $1$845YBKfF$KRXc5AMVJ0CtgpGVr7.nB/
    ssh-authorized-keys:
      - {{.sshkey}}
  - name: unbound
    passwd: $1$845YFGKfF$KRXc5AMVJ0CtgpGVr7.nB/
  - name: consul
    passwd: $1$845YGJfF$KRXc5AMVJ0CtgpGVr7.nB/
coreos: {{if eq .channel "stable"}} 
  etcd:
    name: {{.hostname}}
    addr: {{.ip}}:4001
    bind-addr: 0.0.0.0
    peer-addr: {{.ip}}:7001
{{if ne .role "master"}}
    peers: {{.peers}}
{{end}}
    peer-heartbeat-interval: 250
    peer-election-timeout: 1000{{else}}
  etcd2:
    name: {{.hostname}}
    advertise-client-urls: http://{{.ip}}:2379,http://{{.ip}}:4001
    initial-advertise-peer-urls: http://{{.ip}}:2380
    listen-client-urls: http://0.0.0.0:2379,http://0.0.0.0:4001
    listen-peer-urls: http://{{.ip}}:2380,http://{{.ip}}:7001
    initial-cluster: {{.etcd2peers}}
{{end}}
  fleet:
    public-ip: {{.ip}}
  flannel:
    etcd_prefix: /coreos.com/network
  units:
    - name: cbr0.netdev
      command: start
      content: |
        [NetDev]
        Kind=bridge
        Name=cbr0
    - name: vxlan0.netdev
      command: start
      content: |
        [NetDev]
        Kind=vxlan
        Name=vxlan0

        [VXLAN]
        Id=3989
        Group=224.0.0.1
    - name: vxlan0.network
      command: start
      content: |
        [Match]
        Name=vxlan0

        [Network]
        Bridge=cbr0
    - name: cbr0.network
      command: start
      content: |
        [Match]
        Name=cbr0

        [Network]
        Address={{.vxlan_ip}}/24

        [Route]
        Destination=/22{{if.ext_ip}}
    - name: static.network
      command: start
      content: |
        [Match]
        Name=ens192

        [Network]
        Address={{.ip}}/27
        Gateway={{.gateway}}
        DNS=127.0.0.1
        VXLAN=vxlan0
    - name: extstatic.network
      command: start
      content: |
        [Match]
        Name=ens32

        [Network]
        Address={{.ext_ip}}/27
        Gateway={{.ext_gateway}}
        DNS=127.0.0.1{{else}}
    - name: static.network
      command: start
      content: |
        [Match]
        Name=ens192

        [Network]
        Address={{.ip}}/27
        Gateway={{.gateway}}
        DNS=127.0.0.1
        VXLAN=vxlan0 {{end}}
    - name: etcd2.service
      command: start
    - name: ntpd.service
      command: start
    - name: flanneld.service
      drop-ins:
        - name: 50-network-config.conf
          content: |
            [Service]
            Type=notify
            ExecStartPre=/usr/bin/etcdctl set /coreos.com/network/config '{ "Network": "172.16.0.0/16", "Backend": { "Type": "vxlan", "Port": 8472 }}'
            NotifyAccess=all
      command: start
    - name: fleet.service
      command: start
    - name: unbound.service
      command: start
      content: |
        [Unit]
        After=network.target
        Before=early-docker.target
        Description=Start the unbound dns service
        Documentation=http://www.unbound.net

        [Service]
        PIDFile=/run/unbound.pid
        Environment="ROOT_TRUST_ANCHOR_UPDATE=false,ROOT_TRUST_ANCHOR_FILE=,RESOLVCONF=true"
        ExecStartPre=-/bin/bash -c "test -d /root/configdrive || cp -pruf /media/configdrive /root"
        ExecStart=/bin/bash -c "cd /root/configdrive/unbound/cde-root/root ; /root/configdrive/unbound/cde-root/root/unbound.cde -d -vvvvvvvvv -c /var/run/unbound.conf"
        ExecReload=/bin/kill -9 $MAINPID
        Restart=always
        
        [Install]
        WantedBy=multi-user.target
        RequiredBy=flanneld.service
        RequiredBy=early-docker.service
        RequiredBy=docker.service
        RequiredBy=etcd2.service
        RequiredBy=fleet.service
    - name: diskinit.service
      command: start
      enable: true
      content: |
        [Unit]
        Description=Disk Initialization
        Before=docker.service

        [Service]
        Type=oneshot
        ExecStart=/bin/bash -c "/opt/bin/diskinit.sh"{{if.disk1}}
    - name: format-disk1.service
      command: start
      content: |
        [Unit]
        Description=Formats the drive

        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/bin/bash -c "blkid /dev/sdb | grep ext4 || /usr/sbin/mkfs.ext4 /dev/sdb"
    - name: var-lib-docker.mount
      command: start
      content: |
        [Unit]
        Description=Mount data
        Requires=format-disk1.service
        Requires=diskinit.service
        After=format-disk1.service
        Before=docker.service

        [Mount]
        What=/dev/sdb
        Where=/var/lib/docker
        Type=ext4
        User=root
        Group=root{{end}}{{if.disk2}}
    - name: format-disk2.service
      command: start
      content: |
        [Unit]
        Description=Formats the ephemeral drive

        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/bin/bash -c "blkid /dev/sdc | grep btrfs || /usr/sbin/mkfs.btrfs /dev/sdc"
    - name: data.mount
      command: start
      content: |
        [Unit]
        Description=Mount Backups
        Requires=format-disk2.service
        Requires=diskinit.service
        After=format-disk2.service
        Before=docker.service

        [Mount]
        What=/dev/sdc
        Where=/data
        Type=btrfs
        User=root
        Group=docker{{end}}{{if.disk3}}
    - name: format-disk3.service
      command: start
      content: |
        [Unit]
        Description=Formats the ephemeral drive
        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/bin/bash -c "blkid /dev/sdd | grep btrfs || /usr/sbin/mkfs.btrfs /dev/sdd"
    - name: backups.mount
      command: start
      content: |
        [Unit]
        Description=Mount Backups
        Requires=format-disk3.service
        Requires=diskinit.service
        After=format-disk3.service
        Before=docker.service
        
        [Mount]
        What=/dev/sdd
        Where=/backups
        Type=btrfs
        User=root
        Group=docker{{end}}
    - name: setntp.service
      command: start
      content: |
        [Unit]
        Description=Set NTP on
        Before=ntpd.service

        [Service]
        ExecStart=/usr/bin/timedatectl set-ntp on
        RemainAfterExit=yes
        Type=oneshot
    - name: docker.service
      command: start
      content: |
        [Unit]
        After=network.target
        Requires=docker.socket
        After=docker.socket
        Requires=flanneld.service
        Description=Docker Application Container Engine
        Documentation=http://docs.docker.io

        [Service]
        Type=notify
        EnvironmentFile=/run/flannel_docker_opts.env
        ExecStartPre=/bin/mount --make-rprivate /
        ExecStart=/usr/bin/docker -d -s=overlay -H unix://var/run/docker.sock -H tcp://{{.ip}}:2376 --iptables=false --dns={{.ip}} ${DOCKER_OPT_BIP} ${DOCKER_OPT_IPMASQ} ${DOCKER_OPT_MTU}
        Restart=on-failure
        NotifyAccess=all

        [Install]
        WantedBy=multi-user.target
    - name: copy-cloudconfig1.service
      command: start
      content: |
        [Unit]
        Description=Copy Cloud Config
        After=docker.service flanneld.service

        [Service]
        Type=oneshot
        EnvironmentFile=-/etc/environment
        ExecStartPre=-/bin/bash -c "test -d /root/configdrive || cp -pruf /media/configdrive /root"
        ExecStart=/usr/bin/systemctl enable user-cloudconfigfile.service
        RemainAfterExit=true

        [Install]
        WantedBy=multi-user.target
    - name: docker.path
      command: start
      content: |
        [Unit]
        Description=Environment file reload for docker

        [Path]
        PathExists=/run/flannel_docker_opts.env
        PathChanged=/run/flannel_docker_opts.env
        Unit=docker.service

        [Install]
        RequiredBy=docker.service
  update:
    group: beta
    reboot-strategy: off
ssh_authorized_keys:
  - {{.sshkey}}`))
