# fleet-unit-replicator
Dynamically schedule fleet units based on available fleet machines


## Concepts

 * __delete cooldown time__: To prevent the replicator is running amok, we have a cooldown time for each unit before its getting deleted. By default a unit must be reported as undesired for 60 minutes before we delete it.

 