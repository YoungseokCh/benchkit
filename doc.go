// Package benchkit provides a small generic benchmark framework.
//
// Users define cases, a runner, an oracle, and optionally an aggregator. The
// framework handles bounded parallel execution, per-case timing, interactive
// case selection, and machine-readable JSON or JSONL output.
package benchkit
