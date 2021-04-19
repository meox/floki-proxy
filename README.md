# floki-proxy
![troll](troll.png)

A crazy HTTP proxy that can help you to test the reliabilty of your software

## Examples

- Simulate slow down event from an upstream with a failure-rate of 10%:

```bash
./floki-proxy -failure-rate=10 -failure-code=429
```

- Fail all the requests that starts with `/foo/a/fa/f0b` with a `400` (generic Bad Request)
and all the requests that starts with `/small` with a `500` (Internal Server error).
Use a failure rate of 10% for all requests.

```bash
./floki-proxy -failure-rate=10 -fail-with-prefix="/foo/a/fa/f0b:400;/small:500"
```
