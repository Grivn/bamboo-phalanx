#!/usr/bin/env bash

kill_all_servers(){
    for line in `cat public_ips.txt`
    do
       ssh -t "wanggr"@$line "pkill server ; rm ~/bamboo/server.pid"
    done
}

# distribute files
kill_all_servers
