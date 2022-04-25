#!/usr/bin/env bash

update(){
    for line in `cat real_server.txt`
    do
       scp config.json run.sh ips.txt "root"@$line:~/bphalanx
       ssh "root"@$line 'chmod 777 ~/bphalanx/run.sh'
    done
}

# update config.json to replicas
update
