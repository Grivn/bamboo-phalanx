#!/usr/bin/env bash

kill_all_servers(){
    for line in `cat real_server.txt`
    do
       ssh -t "wanggr"@$line "pkill server ; rm ~/bphalanx/server.pid"
    done
    for line in `cat real_server_2.txt`
    do
       ssh -t "liumm"@$line "pkill server ; rm ~/bphalanx/server.pid"
    done
}

# distribute files
kill_all_servers
