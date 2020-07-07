#!/bin/sh

# Create staging and data directories for nodes
declare -a gopcp_nodes=("node-1" "node-2")

httpPort=10000

for i in "${gopcp_nodes[@]}"
do
    docker run -d --name gopcp-$i \
       -p $((httpPort++)):10002 \
        gopcp:1
done
