# Origin Server

* Create cert file for https requests, put it in the same directory with `main.go`

```shell
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout server.key -out server.crt \
  -days 365 -subj "/CN=localhost"
```

* Start backend (from `/cmd` directory)

```shell
go run main.go
```

* Upload video file for hls process (only after backend is up and alive). You can use what ever video file, test for other extensions (mov, avi).
```shell
curl -k -F "file=@./video_test_1.mp4" https://localhost:8443/api/upload 
```
