# telemetry-test-tools
A repo containing tools developed to assist in the testing of the SUSE
Telemetry service.

## cmd/batcher
The utility runs the specified shell command for a number of iterations,
as specified by the total option, with at most a limited number of command
instances, as specified by the batch option, active at any given time.
