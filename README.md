# fleet-unit-replicator

Dynamically schedule fleet units based on available fleet machines.

The fleet-unit-replicator compiles from the list of machines in fleet and the provided configuration a list of units that needs to be running. This is compared to the list of units in fleet and appropriate actions are taken:

* For new machines, new units are scheduled based on the template
* For existing units, the unitfile is fetched and compared. If unequal, an update is performed when allowed (see Update Cooldown Time)
* If units are detected that have no "living" machine, they are marked for deletion after the deletion cooldown time.


## Concepts

 * __machine tag__: To narrow down the machines that the replicator will schedule units a machine tag can be provided. This is basically the same as using the `MachineMetadata=` direction in unit files.
 * __unit template__: The replicator will use a unit file template provided as a string to schedule units. Machine id tags are added for all hosts matching the provided __machine tag__.
 * __unit prefix__: A prefix that can be used to avoid naming conflicts between mutliple running replicators and other existing units.
 * __delete cooldown time__: To prevent the replicator is running amok, we have a cooldown time for each unit before its getting deleted. By default a unit must be reported as undesired for 60 minutes before we delete it.
 * __update cooldown time__: To prevent the replicator to teardown the whole data services at once and thus rendering everything offline, a cooldown time is applied between updates. Detected updates while within a cooldown phase are ignored.

## Using it

To let the unit replicator schedule units for you based on `fleet` machine
metadata tags use the following unit file definition:
```
[Unit]
Description=replication-manager

[Service]
EnvironmentFile=/etc/environment
Environment="NAME=%n"
Environment="IMAGE=giantswarm/fleet-unit-replicator"
Environment=TEMPLATE_FILE=/tmp/fleet-unit-replicator-%p.tmp
Restart=always
RestartSec=15sec
ExecStartPre=-/usr/bin/docker rm -f $NAME

ExecStartPre=-/bin/rm -rf ${TEMPLATE_FILE}
ExecStartPre=/bin/bash -c "echo '<your unit file>' > ${TEMPLATE_FILE}"

ExecStart=/usr/bin/docker run --rm --name $NAME         -v
${TEMPLATE_FILE}:${TEMPLATE_FILE}    -v /var/run/fleet.sock:/var/run/fleet.sock
$IMAGE          --machine-tag=role-worker      --unit-prefix=your-unit-prefix
--unit-template=@${TEMPLATE_FILE}      --fleet-peers=file:///var/run/fleet.sock
--update-cooldown-time=10m0s    --dry-run=false         --metrics-prefix=%p
--metrics-type=local
ExecStop=-/usr/bin/docker stop -t 10 $NAME
ExecStopPost=-/usr/bin/docker rm -f $NAME
ExecStopPost=-/bin/rm ${TEMPLATE_FILE}
```

