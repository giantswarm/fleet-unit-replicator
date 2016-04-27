# fleet-unit-replicator

Dynamically schedule fleet units based on available fleet machines.

The fleet-unit-replicator compiles from the list of machines in fleet and the provided configuration a list of units that needs to be running. This is compared to the list of units in fleet and appropriate actions are taken:

* For new machines, new units are scheduled based on the template immediately
* For existing units, the unitfile is fetched and compared. If unequal, an update is performed when allowed (see Update Cooldown Time)
* If units are detected that have no "living" machine, they are marked for deletion after the deletion cooldown time.

If you are confident the units are running already and you don't want to wait until `cooldown time` is over, send a `SIGUSR1` to the fleet-unit-replicator, to reset the timer. Upon the next tick, the next unit file will be updated.


## Concepts

 * __delete cooldown time__: To prevent the replicator is running amok, we have a cooldown time for each unit before its getting deleted. By default a unit must be reported as undesired for 60 minutes before we delete it.

 * __update cooldown time__: To prevent the replicator to teardown the whole data services at once and thus rendering everything offline, a cooldown time is applied between updates. Detected updates while within a cooldown phase are ignored.
