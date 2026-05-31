# Send2NLM Scripts

Place producer scripts in `~/.send2nlm/producer/` and receiver scripts in `~/.send2nlm/receiver/`.

Each producer script must export:

```go
var Producer sdk.Producer = &MyProducer{}
```

Each receiver script must export:

```go
var Receiver sdk.Receiver = &MyReceiver{}
```
