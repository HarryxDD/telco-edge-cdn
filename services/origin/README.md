# Origin Server

* Create cert file for https requests, put it in the same directory with `main.go`

```shell
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout server.key -out server.crt \
  -days 365 -subj "/CN=localhost"
```
