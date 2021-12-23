#!/usr/bin/env bash

update(){
    for line in `cat real_server.txt`
    do
       scp config.json run.sh ips.txt "wanggr"@$line:~/bphalanx
       ssh "wanggr"@$line 'chmod 777 ~/bphalanx/run.sh'
    done
    for line in `cat real_server_2.txt`
    do
       scp config.json run.sh ips.txt "liumm"@$line:~/bphalanx
       ssh "liumm"@$line 'chmod 777 ~/bphalanx/run.sh'
    done
}

# update config.json to replicas
update
