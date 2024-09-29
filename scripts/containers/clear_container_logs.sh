for container in /var/lib/docker/containers/*; do
    sudo truncate -s 0 "$container/$(basename $container)-json.log"
done
