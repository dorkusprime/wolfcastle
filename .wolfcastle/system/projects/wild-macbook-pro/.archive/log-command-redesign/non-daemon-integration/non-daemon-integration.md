# Non-Daemon Integration

Wire the logrender interleaved renderer into wolfcastle execute and wolfcastle intake non-daemon modes. When running outside the daemon, these commands should stream interleaved-format output to stdout in real time: a goroutine tails the NDJSON log file the command is writing to and renders the interleaved view using the shared logrender package. The daemon itself continues writing raw NDJSON; the rendering goroutine is the consumer.
