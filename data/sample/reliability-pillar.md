# Reliability

## Distribute across Availability Zones

Run stateless application servers in at least two Availability Zones behind a load balancer. Use a managed database with a standby replica in a different zone, so that the loss of a single zone does not take the service down. Health checks should remove failing instances from rotation automatically.

## Back up and test recovery

Enable automated backups and point-in-time recovery for every data store, and copy critical backups to another region when the recovery objectives require it. Rehearse restores on a schedule. A backup that has never been restored is an assumption, not a recovery plan.

## Design for failure and limit the blast radius

Assume that components fail. Use timeouts, retries with exponential backoff and jitter, and circuit breakers when calling dependencies. Isolate workloads so that a failure or traffic spike in one part does not exhaust shared resources, and set quotas with headroom for growth.

## Define recovery objectives

Agree on a recovery time objective and a recovery point objective with the business before choosing a disaster recovery strategy. Backup and restore, pilot light, warm standby, and active-active trade cost against recovery speed.
