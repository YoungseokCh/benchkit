# Incremental Example

This example shows that `-update` reruns only selected cases while preserving
the rest of the previous whole-suite result.

Run the full suite into a chosen result directory:

```sh
go run ./example/incremental -tui=false -result-dir /tmp/benchkit-incremental-demo
```

Then rerun only `beta` with a different score:

```sh
SCORE_beta=200 go run ./example/incremental -tui=false -result-dir /tmp/benchkit-incremental-demo -update -case beta
```

Inspect the merged result:

```sh
cat /tmp/benchkit-incremental-demo/summary.json
```

Expected result:

- `alpha` still has score `10`.
- `beta` has score `200`.
- `gamma` still has score `30`.
- the aggregate total is `240`.
