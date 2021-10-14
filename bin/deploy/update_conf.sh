#!/usr/bin/env bash

update(){
    for line in `cat public_ips.txt`
    do
       scp config.json run.sh ips.txt "wanggr"@$line:~/bamboo
       ssh "wanggr"@$line 'chmod 777 ~/bamboo/run.sh'
    done
}

# update config.json to replicas
update
