#!/usr/bin/env bash
sh ./pkill.sh

start(){
  count=0
  for line in `cat public_ips.txt`
  do
    count=$((count+1))
    ssh -t "wanggr"@$line "cd ~/bphalanx ; nohup ./run.sh $count"
    sleep 0.1
    echo replica $count is launched!
  done
  for line in `cat public_ips_2.txt`
  do
    count=$((count+1))
    ssh -t "liumm"@$line "cd ~/bphalanx ; nohup ./run.sh $count"
    sleep 0.1
    echo replica $count is launched!
  done
}

# update config.json to replicas
start
