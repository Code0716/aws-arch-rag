# Performance Efficiency

## Cache close to the user

Place a cache and a content delivery network in front of read-heavy paths so that most requests are answered without reaching the database. Cache at several layers and define clear expiry rules.

## Choose resources that fit the workload

Pick compute, storage and database types by the access pattern of the workload, not by habit. Serverless and managed services remove capacity planning for spiky traffic, while sustained high load often favors provisioned capacity.

## Measure, then tune

Define latency and throughput targets, load test before launch, and monitor percentiles rather than averages. Change one thing at a time and keep the result of each experiment.

## Reduce distance and round trips

Deploy close to users, keep chatty services in the same zone when possible, and use connection pooling and asynchronous processing for work that does not need an immediate answer.
