#!/usr/bin/env bash

distribute(){
    for line in `cat clients.txt`
    do
       ssh "wanggr"@$line "mkdir bphalanx"
       echo "---- upload client: wanggr@$line\n ----"
       scp client ips.txt config.json runClient.sh closeClient.sh "wanggr"@$line:~/bphalanx
       ssh "wanggr"@$line "chmod 777 ~/bphalanx/runClient.sh"
       ssh "wanggr"@$line "chmod 777 ~/bphalanx/closeClient.sh"
    done
}

# distribute files
distribute
