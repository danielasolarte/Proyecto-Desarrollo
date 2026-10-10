#!/usr/bin/env bash
# Muestrea cada INT segundos, durante DUR segundos, la cola asynq 'media' (si
# Redis corre en esta VM), la memoria y carga de la VM y CPU/memoria de los
# contenedores worker, clamav, redis y api. Escribe un CSV en OUT.
DUR=${1:-1800}; INT=${2:-5}; OUT=${3:-/tmp/monitor.csv}
R=$(sudo docker ps -q --filter label=com.docker.compose.service=redis | head -1)
echo timestamp_utc,pending,active,scheduled,retry,archived,completed,vm_mem_used_mb,vm_mem_avail_mb,load1,worker_cpu,worker_mem,clamav_cpu,clamav_mem,redis_cpu,redis_mem,api_cpu,api_mem > $OUT
q() { if [ -n "$R" ]; then sudo docker exec $R redis-cli $1 asynq:{media}:$2 2>/dev/null || echo 0; else echo 0; fi; }
c() { echo "$ST" | grep -- "$1" | head -1 | awk -F';' '{split($3,m," "); print $2","m[1]}'; }
end=$(( $(date +%s) + DUR ))
while [ $(date +%s) -lt $end ]; do
  ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  p=$(q LLEN pending); a=$(q LLEN active); s=$(q ZCARD scheduled); r=$(q ZCARD retry); ar=$(q ZCARD archived); co=$(q ZCARD completed)
  read used avail <<< $(free -m | awk '/Mem:/{print $3, $7}')
  load=$(cut -d' ' -f1 /proc/loadavg)
  ST=$(sudo docker stats --no-stream --format '{{.Name}};{{.CPUPerc}};{{.MemUsage}}')
  w=$(c worker); cl=$(c clamav); re=$(c redis); ap=$(c -api-)
  echo $ts,$p,$a,$s,$r,$ar,$co,$used,$avail,$load,${w:-,},${cl:-,},${re:-,},${ap:-,} >> $OUT
  sleep $INT
done
