FROM busybox:ubuntu-14.04

COPY fleet-unit-replicator /opt/fleet-unit-replicator

ENTRYPOINT ["/opt/fleet-unit-replicator"]
