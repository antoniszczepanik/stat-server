# stat-server

Precisely track most recent resource usage information on remote host and serve
the result via very simple API.

For now it only supports avg CPU usage, however extending it should be
trivial. PRs are welcome.

1. Run it on a target server:

```sh
# Poll CPU with frequency of 100ms, limit history to 60s
$ ./stat-server -f 100 -l 60
```

2. Request the data.

```sh
$ # Get all datapoints from last 300 ms.
$ curl "http://localhost:2137?ms=300" | jq
[
  {
    "ts": 1654259802170371800,
    "value": 6.17283950654989
  },
  {
    "ts": 1654259802270917600,
    "value": 3.846153846010336
  },
  {
    "ts": 1654259802371504000,
    "value": 3.750000000363798
  }
]
```

`ts` is a time elapsed since the Unix epoch in nanoseconds
`value` is % cpu consumption

I wrote this because I needed a way to ask a remote server about it's
precise CPU consumption in last 500ms.
