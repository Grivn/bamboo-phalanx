#!/usr/bin/env bash

distribute(){
    for line in `cat clients.txt`
    do
       ssh "root"@$line "mkdir bphalanx"
       echo "---- upload client: root@$line\n ----"
       scp ips.txt config.json "root"@$line:~/bphalanx
       ssh "root"@$line "chmod 777 ~/bphalanx/runClient.sh"
       ssh "root"@$line "chmod 777 ~/bphalanx/closeClient.sh"
    done
}

# distribute files
distribute
