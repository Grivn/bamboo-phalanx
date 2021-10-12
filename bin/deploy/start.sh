#!/usr/bin/env bash
./pkill.sh

start(){
    SERVER_ADDR=(`cat public_ips.txt`)
    for (( j=1; j<=$1; j++))
    do
      ssh -t $2@${SERVER_ADDR[j-1]} "cd /home/${2}/bamboo ; nohup ./run.sh ${j}"
      sleep 0.1
      echo replica ${j} is launched!
    done
}

USERNAME="wanggr"
MAXPEERNUM=(`wc -l public_ips.txt | awk '{ print $1 }'`)

# update config.json to replicas
start $MAXPEERNUM $USERNAME
