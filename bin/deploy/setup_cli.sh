#!/usr/bin/env bash

distribute(){
    for line in `cat clients.txt`
    do
       ssh "wanggr"@$line "mkdir bamboo"
       echo "---- upload client: wanggr@$line\n ----"
       scp client ips.txt config.json runClient.sh closeClient.sh "wanggr"@$line:~/bamboo
       ssh "wanggr"@$line "chmod 777 ~/bamboo/runClient.sh"
       ssh "wanggr"@$line "chmod 777 ~/bamboo/closeClient.sh"
    done
}

# distribute files
distribute
