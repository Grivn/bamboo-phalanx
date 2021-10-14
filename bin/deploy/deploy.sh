#!/usr/bin/env bash

distribute(){
    for line in `cat public_ips.txt`
    do 
      ssh "wanggr"@$line "mkdir ~/bamboo"
      echo "---- upload replica: wanggr@$line \n ----"
      scp server ips.txt run.sh "wanggr"@$line:~/bamboo
    done
}

# distribute files
distribute
