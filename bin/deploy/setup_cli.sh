#!/usr/bin/env bash

distribute(){
    for line in `cat clients.txt`
    do
       ssh "root"@$line "mkdir bphalanx"
       echo "---- upload client: root@$line\n ----"
       scp client ips.txt config.json runClient.sh closeClient.sh "root"@$line:~/bphalanx
       ssh "root"@$line "chmod 777 ~/bphalanx/runClient.sh"
       ssh "root"@$line "chmod 777 ~/bphalanx/closeClient.sh"
    done
}

# distribute files
distribute
