# fleet-unit-replicator
Dynamically schedule fleet units based on available fleet machines


## Concepts

 * __delete cooldown time__: To prevent the replicator is running amok, we have a cooldown time for each unit before its getting deleted. By default a unit must be reported as undesired for 60 minutes before we delete it.

 * __update cooldown time__: To prevent the replicator to teardown the whole data services at once and thus rendering everything offline, a cooldown time is applied between updates. Detected updates while within a cooldown phase are ignored.

 