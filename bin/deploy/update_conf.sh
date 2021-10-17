#!/usr/bin/env bash

update(){
    for line in `cat public_ips.txt`
    do
       scp config.json run.sh ips.txt "wanggr"@$line:~/bphalanx
       ssh "wanggr"@$line 'chmod 777 ~/bphalanx/run.sh'
    done
}

# update config.json to replicas
update
