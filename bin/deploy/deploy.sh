#!/usr/bin/env bash
sh ./pkill.sh

distribute(){
    for line in `cat real_server.txt`
    do
      ssh "root"@$line "mkdir ~/bphalanx"
      echo "---- upload replica: root@$line \n ----"
      scp server "root"@$line:~/bphalanx
    done
}

# distribute files
distribute
